// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var connectionString = flag.String(
	"conn",
	"postgresql://root@localhost:26257/defaultdb?sslmode=disable",
	"cockroach connection string",
)
var port = flag.Int("port", 26258, "http server listening port")

var sinkDB = flag.String("sink_db", "_CDC_SINK", "db for storing temp sink tables")
var dropDB = flag.Bool("drop", false, "Drop the sink db before starting?")
var sinkDBZone = flag.Bool(
	"sink_db_zone_override",
	true,
	"allow sink_db zone config to be overridden with the cdc-sink default values",
)

var configuration = flag.String(
	"config",
	"",
	`This flag must be set. It requires a single line for each table passed in.
The format is the following:
[
	{"endpoint":"", "source_table":"", "destination_database":"", "destination_table":""},
	{"endpoint":"", "source_table":"", "destination_database":"", "destination_table":""},
]

Each table being updated requires a single line. Note that source database is
not required.
Each changefeed requires the same endpoint and you can have more than one table
in a single changefeed.

Here are two examples:

1) Single table changefeed.  Source table and destination table are both called
users:

[{endpoint:"cdc.sql", source_table:"users", destination_database:"defaultdb", destination_table:"users"}]

The changefeed is initialized on the source database:
CREATE CHANGEFEED FOR TABLE users INTO 'experimental-[cdc-sink-url:port]/cdc.sql' WITH updated,resolved

2) Two table changefeed. Two tables this time, users and customers:

[
	{"endpoint":"cdc.sql", "source_table":"users", "destination_database":"defaultdb", "destination_table":"users"},
	{"endpoint":"cdc.sql", "source_table":"customers", "destination_database":"defaultdb", "destination_table":"customers"},
]

The changefeed is initialized on the source database:
CREATE CHANGEFEED FOR TABLE users,customers INTO 'experimental-[cdc-sink-url:port]/cdc.sql' WITH updated,resolved

As of right now, only a single endpoint is supported.

Don't forget to escape the json quotes:
./cdc-sink --config="[{\"endpoint\":\"test.sql\", \"source_table\":\"in_test1\", \"destination_database\":\"defaultdb\", \"destination_table\":\"out_test1\"},{\"endpoint\":\"test.sql\", \"source_table\":\"in_test2\", \"destination_database\":\"defaultdb\", \"destination_table\":\"out_test2\"}]"`,
)

var (
	// buildVersion is set by the go linker at build time
	buildVersion = "<unknown>"
	printVersion = flag.Bool("version", false, "print version and exit")
)

func createHandler(db *pgxpool.Pool, sinks *Sinks) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Is it an ndjson url?
		ndjson, ndjsonErr := parseNdjsonURL(r.RequestURI)
		if ndjsonErr == nil {
			sink := sinks.FindSink(ndjson.endpoint, ndjson.topic)
			if sink != nil {
				sink.HandleRequest(db, w, r)
				return
			}

			// No sink found, throw an error.
			http.Error(
				w,
				fmt.Sprintf("could not find a sync for %s", ndjson.topic),
				http.StatusInternalServerError,
			)
			return
		}

		// Is it a resolved url?
		resolved, resolvedErr := parseResolvedURL(r.RequestURI)
		if resolvedErr == nil {
			sinks.HandleResolvedRequest(r.Context(), db, resolved, w, r)
			return
		}

		// Could not recognize url.
		http.Error(
			w,
			fmt.Sprintf("URL pattern does not match either an ndjson (%s) or a resolved (%s)",
				ndjsonErr, resolvedErr,
			),
			http.StatusInternalServerError,
		)
		return
	}
}

// Config parses the passed in config.
type Config []ConfigEntry

// ConfigEntry is a single table configuration entry in a config.
type ConfigEntry struct {
	Endpoint            string `json:"endpoint"`
	SourceTable         string `json:"source_table"`
	DestinationDatabase string `json:"destination_database"`
	DestinationTable    string `json:"destination_table"`
}

func parseConfig(rawConfig string) (Config, error) {
	var config Config
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return Config{}, fmt.Errorf("Could not parse config: %s", err.Error())
	}

	if len(config) == 0 {
		return Config{}, fmt.Errorf("No config lines provided")
	}

	for _, entry := range config {
		if len(entry.Endpoint) == 0 {
			return Config{}, fmt.Errorf("Each config entry requires and endpoint")
		}

		if len(entry.SourceTable) == 0 {
			return Config{}, fmt.Errorf("Each config entry requires a source_table")
		}

		if len(entry.DestinationDatabase) == 0 {
			return Config{}, fmt.Errorf("Each config entry requires a destination_database")
		}

		if len(entry.DestinationTable) == 0 {
			return Config{}, fmt.Errorf("Each config entry requires a destination_table")
		}
	}

	return config, nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	// First, parse the config.
	flag.Parse()

	if *printVersion {
		fmt.Println("cdc-sink", buildVersion)
		fmt.Println(runtime.Version(), runtime.GOARCH, runtime.GOOS)
		fmt.Println()
		if bi, ok := debug.ReadBuildInfo(); ok {
			fmt.Println(bi.Main.Path, bi.Main.Version)
			for _, m := range bi.Deps {
				for m.Replace != nil {
					m = m.Replace
				}
				fmt.Println(m.Path, m.Version)
			}
		}
		return
	}

	config, err := parseConfig(*configuration)
	if err != nil {
		log.Print(*configuration)
		log.Fatal(err)
	}

	db, err := pgxpool.Connect(ctx, *connectionString)
	if err != nil {
		log.Fatalf("could not parse config string: %v", err)
	}
	defer db.Close()

	if *dropDB {
		if err := DropSinkDB(ctx, db); err != nil {
			log.Fatalf("Could not drop the sinkDB:%s - %v", *sinkDB, err)
		}
	}

	if err := CreateSinkDB(ctx, db); err != nil {
		log.Fatalf("Could not create the sinkDB:%s - %v", *sinkDB, err)
	}

	sinks, err := CreateSinks(ctx, db, config)
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("could not open listener: %v", err)
	}
	log.Printf("listening on %s", l.Addr())

	handler := http.Handler(http.HandlerFunc(createHandler(db, sinks)))
	handler = h2c.NewHandler(handler, &http2.Server{})

	// TODO(bob): Consider configuring timeouts
	svr := &http.Server{Handler: handler}
	go svr.Serve(l)
	<-ctx.Done()
	log.Printf("waiting for connections to drain")
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	_ = svr.Shutdown(ctx)
	cancel()
}
