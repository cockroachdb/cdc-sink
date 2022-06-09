// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cdc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/auth/reject"
	"github.com/cockroachdb/cdc-sink/internal/target/auth/trust"
	"github.com/cockroachdb/cdc-sink/internal/target/leases"
	"github.com/cockroachdb/cdc-sink/internal/target/resolve"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/target/sinktest"
	"github.com/cockroachdb/cdc-sink/internal/target/stage"
	"github.com/cockroachdb/cdc-sink/internal/target/timekeeper"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {
	t.Run("deferred", func(t *testing.T) { testHandler(t, false) })
	t.Run("immediate", func(t *testing.T) { testHandler(t, true) })
}

func testHandler(t *testing.T, immediate bool) {
	t.Helper()
	a := assert.New(t)

	ctx, dbInfo, cancel := sinktest.Context()
	defer cancel()

	dbName, cancel, err := sinktest.CreateDB(ctx)
	if !a.NoError(err) {
		return
	}
	defer cancel()

	tableInfo, err := sinktest.CreateTable(ctx, dbName,
		`CREATE TABLE %s (pk INT PRIMARY KEY, v INT NOT NULL)`)
	if !a.NoError(err) {
		return
	}

	swapper, cancel, err := timekeeper.NewTimeKeeper(ctx, dbInfo.Pool(), Resolved)
	if !a.NoError(err) {
		return
	}
	defer cancel()

	watchers, cancel := schemawatch.NewWatchers(dbInfo.Pool())
	defer cancel()

	appliers, cancel := apply.NewAppliers(watchers)
	defer cancel()

	lss, err := leases.New(ctx, leases.Config{
		Pool:     dbInfo.Pool(),
		Target:   leases.Table,
		Lifetime: time.Minute,
		Poll:     time.Second,
	})
	if !a.NoError(err) {
		return
	}

	stagers := stage.NewStagers(dbInfo.Pool(), ident.StagingDB)

	resolvers, cancel, err := resolve.New(ctx, resolve.Config{
		Appliers:   appliers,
		Leases:     lss,
		MetaTable:  resolve.Table,
		Pool:       dbInfo.Pool(),
		Stagers:    stagers,
		Timekeeper: swapper,
		Watchers:   watchers,
	})
	if !a.NoError(err) {
		return
	}
	defer cancel()

	h := &Handler{
		Appliers:      appliers,
		Authenticator: trust.New(),
		Pool:          dbInfo.Pool(),
		Resolvers:     resolvers,
		Stores:        stagers,
		Swapper:       swapper,
		Watchers:      watchers,
	}

	// In async mode, we want to reach into the implementation to
	// force the marked, resolved timestamp to be operated on.
	maybeFlush := func(target ident.Schematic) error {
		if immediate {
			return nil
		}
		resolver, err := h.Resolvers.Get(ctx, target.AsSchema())
		if err != nil {
			return err
		}
		_, _, err = resolver.Flush(ctx)
		return err
	}

	// Validate the "classic" endpoints where files containing
	// newline-delimited json and resolved timestamp markers are
	// uploaded.
	//
	// The inner methods of Handler are called directly, to avoid the
	// need to cook up fake http.Request instances.
	t.Run("bulkStyleEndpoints", func(t *testing.T) {
		a := assert.New(t)

		// Stage two lines of data.
		a.NoError(h.ndjson(ctx, &request{
			immediate: immediate,
			target:    tableInfo.Name(),
			body: strings.NewReader(`
{ "after" : { "pk" : 42, "v" : 99 }, "key" : [ 42 ], "updated" : "1.0" }
{ "after" : { "pk" : 99, "v" : 42 }, "key" : [ 99 ], "updated" : "1.0" }
`),
		}))

		// Send the resolved timestamp request.
		a.NoError(h.resolved(ctx, &request{
			immediate: immediate,
			target:    tableInfo.Name(),
			timestamp: hlc.New(2, 0),
		}))
		a.NoError(maybeFlush(tableInfo.Name()))

		ct, err := tableInfo.RowCount(ctx)
		a.NoError(err)
		a.Equal(2, ct)

		// Now, delete the data.
		a.NoError(h.ndjson(ctx, &request{
			target:    tableInfo.Name(),
			immediate: immediate,
			body: strings.NewReader(`
{ "after" : null, "key" : [ 42 ], "updated" : "3.0" }
{ "key" : [ 99 ], "updated" : "3.0" }
`),
		}))

		a.NoError(h.resolved(ctx, &request{
			immediate: immediate,
			target:    tableInfo.Name(),
			timestamp: hlc.New(5, 0),
		}))
		a.NoError(maybeFlush(tableInfo.Name()))

		ct, err = tableInfo.RowCount(ctx)
		a.NoError(err)
		a.Equal(0, ct)
	})

	// This test validates the new webhook-https:// scheme introduced
	// in CockroachDB v21.2. We insert rows and delete them.
	t.Run("webhookEndpoints", func(t *testing.T) {
		a := assert.New(t)
		schema := ident.NewSchema(tableInfo.Name().Database(), tableInfo.Name().Schema())

		// Insert data and verify flushing.
		a.NoError(h.webhook(ctx, &request{
			immediate: immediate,
			target:    schema,
			body: strings.NewReader(fmt.Sprintf(`
{ "payload" : [
  { "after" : { "pk" : 42, "v" : 99 }, "key" : [ 42 ], "topic" : %[1]s, "updated" : "10.0" },
  { "after" : { "pk" : 99, "v" : 42 }, "key" : [ 99 ], "topic" : %[1]s, "updated" : "10.0" }
] }
`, tableInfo.Name().Table())),
		}))

		a.NoError(h.webhook(ctx, &request{
			immediate: immediate,
			target:    schema,
			body:      strings.NewReader(`{ "resolved" : "20.0" }`),
		}))
		a.NoError(maybeFlush(schema))

		ct, err := tableInfo.RowCount(ctx)
		a.NoError(err)
		a.Equal(2, ct)

		// Now, delete the data.
		a.NoError(h.webhook(ctx, &request{
			immediate: immediate,
			target:    schema,
			body: strings.NewReader(fmt.Sprintf(`
{ "payload" : [
  { "key" : [ 42 ], "topic" : %[1]s, "updated" : "30.0" },
  { "after" : null, "key" : [ 99 ], "topic" : %[1]s, "updated" : "30.0" }
] }
`, tableInfo.Name().Table())),
		}))

		a.NoError(h.webhook(ctx, &request{
			immediate: immediate,
			target:    schema,
			body:      strings.NewReader(`{ "resolved" : "40.0" }`),
		}))
		a.NoError(maybeFlush(schema))

		ct, err = tableInfo.RowCount(ctx)
		a.NoError(err)
		a.Equal(0, ct)
	})

	// Verify that an empty post doesn't crash.
	t.Run("empty-ndjson", func(t *testing.T) {
		a := assert.New(t)
		a.NoError(h.ndjson(ctx, &request{
			immediate: immediate,
			target:    tableInfo.Name(),
			body:      strings.NewReader(""),
		}))
	})

	// Verify that an empty webhook post doesn't crash.
	t.Run("empty-webhook", func(t *testing.T) {
		a := assert.New(t)
		a.NoError(h.webhook(ctx, &request{
			immediate: immediate,
			target:    ident.NewSchema(tableInfo.Name().Database(), tableInfo.Name().Schema()),
			body:      strings.NewReader(""),
		}))
	})

	// Advance the resolved timestamp for the table to well into the
	// future and then attempt to roll it back.
	t.Run("resolved-goes-backwards", func(t *testing.T) {
		a := assert.New(t)

		a.NoError(h.resolved(ctx, &request{
			immediate: immediate,
			target:    ident.NewSchema(tableInfo.Name().Database(), tableInfo.Name().Schema()),
			timestamp: hlc.New(50000, 0),
		}))
		err := h.resolved(ctx, &request{
			immediate: immediate,
			target:    ident.NewSchema(tableInfo.Name().Database(), tableInfo.Name().Schema()),
			timestamp: hlc.New(25, 0),
		})
		if immediate {
			// Resolved timestamp tracking is disabled in immediate mode.
			a.NoError(err)
		} else if a.Error(err) {
			a.Contains(err.Error(), "backwards")
		}
	})
}

func TestRejectedAuth(t *testing.T) {
	// Verify that auth checks don't require other services.
	h := &Handler{
		Authenticator: reject.New(),
	}

	tcs := []struct {
		path string
		code int
	}{
		{"/targetDB/targetSchema/2020-04-02/202004022058072107140000000000000-56087568dba1e6b8-1-72-00000000-test_table-1.ndjson", http.StatusUnauthorized},
		{"/test/public/2020-04-04/202004042351304139680000000000000.RESOLVED", http.StatusUnauthorized},
		{"/some/schema", http.StatusUnauthorized}, // webhook
		{"/", http.StatusNotFound},
	}

	for idx, tc := range tcs {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			a := assert.New(t)
			req := httptest.NewRequest("POST", tc.path, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)
			a.Equal(tc.code, w.Code)
		})
	}
}
