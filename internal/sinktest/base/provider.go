// Copyright 2023 The Cockroach Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package base provides enough functionality to connect to a database,
// but does not provide any other services. This package is primary used
// to break dependency cycles in tests.
package base

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/sinktest"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/retry"
	"github.com/cockroachdb/cdc-sink/internal/util/stdpool"
	"github.com/google/wire"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	defaultConnString = "postgresql://root@localhost:26257/defaultdb?sslmode=disable"

	envSourceString  = "TEST_SOURCE_CONNECT"
	envStagingString = "TEST_STAGING_CONNECT"
	envTargetString  = "TEST_TARGET_CONNECT"
)

var (
	sourceConn   *string
	stagingConn  *string
	targetString *string
)

func init() {
	// We use os.Getenv and a length check so that defined, empty
	// environment variables are ignored.

	sourceConnect := defaultConnString
	if found := os.Getenv(envSourceString); len(found) > 0 {
		sourceConnect = found
	}
	sourceConn = flag.String("testSourceConnect", sourceConnect,
		"the connection string to use for the source db")

	stagingConnect := defaultConnString
	if found := os.Getenv(envStagingString); len(found) > 0 {
		stagingConnect = found
	}
	stagingConn = flag.String("testStagingConnect", stagingConnect,
		"the connection string to use for the staging db")

	targetConnect := defaultConnString
	if found := os.Getenv(envTargetString); len(found) > 0 {
		targetConnect = found
	}
	targetString = flag.String("testTargetConnect", targetConnect,
		"the connection string to use for the target db")
}

// TestSet is used by wire.
var TestSet = wire.NewSet(
	ProvideContext,
	ProvideStagingSchema,
	ProvideStagingPool,
	ProvideSourcePool,
	ProvideSourceSchema,
	ProvideTargetPool,
	ProvideTargetSchema,

	wire.Struct(new(Fixture), "*"),
)

// Fixture can be used for tests that "just need a database",
// without the other services provided by the target package. One can be
// constructed by calling NewFixture.
type Fixture struct {
	Context      context.Context       // The context for the test.
	SourcePool   *types.SourcePool     // Access to user-data tables and changefeed creation.
	SourceSchema sinktest.SourceSchema // A container for tables within SourcePool.
	StagingPool  *types.StagingPool    // Access to __cdc_sink database.
	StagingDB    ident.StagingSchema   // The _cdc_sink SQL DATABASE.
	TargetPool   *types.TargetPool     // Access to the destination.
	TargetSchema sinktest.TargetSchema // A container for tables within TargetPool.
}

// CreateSourceTable creates a test table within the SourcePool and
// SourceSchema. The schemaSpec parameter must have exactly one %s
// substitution parameter for the database name and table name.
func (f *Fixture) CreateSourceTable(
	ctx context.Context, schemaSpec string,
) (TableInfo[*types.SourcePool], error) {
	return CreateTable(ctx, f.SourcePool, f.SourceSchema.Schema(), schemaSpec)
}

// CreateTargetTable creates a test table within the TargetPool and
// TargetSchema. The schemaSpec parameter must have exactly one %s
// substitution parameter for the database name and table name.
func (f *Fixture) CreateTargetTable(
	ctx context.Context, schemaSpec string,
) (TableInfo[*types.TargetPool], error) {
	return CreateTable(ctx, f.TargetPool, f.TargetSchema.Schema(), schemaSpec)
}

var caseTimout = flag.Duration(
	"caseTimout",
	2*time.Minute,
	"raise this value when debugging to allow individual tests to run longer",
)

// ProvideContext returns an execution context that is associated with a
// singleton connection to a CockroachDB cluster.
func ProvideContext() (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), *caseTimout)
	return ctx, cancel, nil
}

// ProvideSourcePool connects to the source database. If the source is a
// CockroachDB cluster, this function will also configure the rangefeed
// and license cluster settings if they have not been previously
// configured.
func ProvideSourcePool(ctx context.Context) (*types.SourcePool, func(), error) {
	tgt := *sourceConn
	log.Infof("source connect string: %s", tgt)
	ret, cancel, err := stdpool.OpenTarget(ctx, tgt)
	if err != nil {
		return nil, nil, err
	}

	if ret.Product != types.ProductCockroachDB {
		return (*types.SourcePool)(ret), cancel, err
	}

	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()

	// Set the cluster settings once, if we need to.
	var enabled bool
	if err := retry.Retry(ctx, func(ctx context.Context) error {
		return ret.QueryRowContext(ctx, "SHOW CLUSTER SETTING kv.rangefeed.enabled").Scan(&enabled)
	}); err != nil {
		return nil, nil, errors.Wrap(err, "could not check cluster setting")
	}
	if !enabled {
		if lic, ok := os.LookupEnv("COCKROACH_DEV_LICENSE"); ok {
			if err := retry.Execute(ctx, ret,
				"SET CLUSTER SETTING cluster.organization = $1",
				"Cockroach Labs - Production Testing",
			); err != nil {
				return nil, nil, errors.Wrap(err, "could not set cluster.organization")
			}
			if err := retry.Execute(ctx, ret,
				"SET CLUSTER SETTING enterprise.license = $1", lic,
			); err != nil {
				return nil, nil, errors.Wrap(err, "could not set enterprise.license")
			}
		}

		if err := retry.Execute(ctx, ret,
			"SET CLUSTER SETTING kv.rangefeed.enabled = true"); err != nil {
			return nil, nil, errors.Wrap(err, "could not enable rangefeeds")
		}
	}

	success = true
	return (*types.SourcePool)(ret), cancel, nil
}

// ProvideStagingSchema create a globally-unique container for tables in the
// staging database.
func ProvideStagingSchema(
	ctx context.Context, pool *types.StagingPool,
) (ident.StagingSchema, func(), error) {
	ret, cancel, err := provideSchema(ctx, pool, "cdc")
	return ident.StagingSchema(ret), cancel, err
}

// ProvideStagingPool opens a connection to the CockroachDB staging
// cluster under test.
func ProvideStagingPool(ctx context.Context) (*types.StagingPool, func(), error) {
	tgt := *stagingConn
	log.Infof("staging connect string: %s", tgt)
	return stdpool.OpenPgxAsStaging(ctx, tgt)
}

// ProvideTargetPool connects to the target database (which is most
// often the same as the source database).
func ProvideTargetPool(
	ctx context.Context, source *types.SourcePool,
) (*types.TargetPool, func(), error) {
	tgt := *targetString
	if tgt == source.ConnectionString {
		log.Info("reusing SourcePool as TargetPool")
		return (*types.TargetPool)(source), func() {}, nil
	}
	log.Infof("target connect string: %s", tgt)
	return stdpool.OpenTarget(ctx, *targetString)
}

// ProvideSourceSchema create a globally-unique container for tables in
// the source database.
func ProvideSourceSchema(
	ctx context.Context, pool *types.SourcePool,
) (sinktest.SourceSchema, func(), error) {
	sch, cancel, err := provideSchema(ctx, pool, "src")
	log.Infof("source schema: %s", sch)
	return sinktest.SourceSchema(sch), cancel, err
}

// ProvideTargetSchema create a globally-unique container for tables in
// the target database.
func ProvideTargetSchema(
	ctx context.Context, pool *types.TargetPool,
) (sinktest.TargetSchema, func(), error) {
	sch, cancel, err := provideSchema(ctx, pool, "tgt")
	log.Infof("target schema: %s", sch)
	return sinktest.TargetSchema(sch), cancel, err
}

func provideSchema[P types.AnyPool](
	ctx context.Context, pool P, prefix string,
) (ident.Schema, func(), error) {
	switch pool.Info().Product {
	case types.ProductCockroachDB, types.ProductPostgreSQL:
		return CreateSchema(ctx, pool, prefix)

	case types.ProductOracle:
		// Each package tests run in a separate binary, so we need a
		// "globally" unique ID.  While PIDs do recycle, they're highly
		// unlikely to do so during a single run of the test suite.
		name := ident.New(fmt.Sprintf(
			"%s_%d_%d", prefix, os.Getpid(), atomic.AddInt32(&dbIdentCounter, 1)))

		err := retry.Execute(ctx, pool, fmt.Sprintf("CREATE USER %s", name))
		if err != nil {
			return ident.Schema{}, nil, errors.Wrapf(err, "could not create user %s", name)
		}

		cancel := func() {
			err := retry.Execute(ctx, pool, fmt.Sprintf("DROP USER %s CASCADE", name))
			if err != nil {
				log.WithError(err).Warnf("could not clean up schema %s", name)
			}
		}

		return ident.MustSchema(name), cancel, nil

	default:
		return ident.Schema{}, nil,
			errors.Errorf("cannot create test db for %s", pool.Info().Product)
	}
}

// Ensure unique database identifiers within a test run.
var dbIdentCounter int32

// CreateSchema creates a schema with a unique name that will be
// dropped when the cancel function is called.
func CreateSchema[P types.AnyPool](
	ctx context.Context, pool P, prefix string,
) (ident.Schema, func(), error) {
	dbNum := atomic.AddInt32(&dbIdentCounter, 1)

	// Each package tests run in a separate binary, so we need a
	// "globally" unique ID.  While PIDs do recycle, they're highly
	// unlikely to do so during a single run of the test suite.
	name := ident.New(fmt.Sprintf("%s_%d_%d", prefix, os.Getpid(), dbNum))

	cancel := func() {
		err := retry.Execute(ctx, pool, fmt.Sprintf("DROP DATABASE IF EXISTS %s CASCADE", name))
		log.WithError(err).WithField("target", name).Debug("dropped database")
	}

	// Clean up if anything below fails.
	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()

	if err := retry.Execute(ctx, pool, fmt.Sprintf(
		"CREATE DATABASE %s", name)); err != nil {
		return ident.Schema{}, cancel, errors.WithStack(err)
	}

	if err := retry.Execute(ctx, pool, fmt.Sprintf(
		`ALTER DATABASE %s CONFIGURE ZONE USING gc.ttlseconds = 600`, name)); err != nil {
		return ident.Schema{}, cancel, errors.WithStack(err)
	}

	sch, err := ident.NewSchema(name, ident.Public)
	if err != nil {
		return ident.Schema{}, cancel, err
	}

	success = true
	return sch, cancel, nil
}

// A global counter for allocating all temp tables in a test run. We
// know that the enclosing database has a unique name, but it's
// convenient for all test table names to be unique as well.
var tempTable int32

// CreateTable creates a test table. The schemaSpec parameter must have
// exactly one %s substitution parameter for the database name and table
// name.
func CreateTable[P types.AnyPool](
	ctx context.Context, pool P, enclosing ident.Schema, schemaSpec string,
) (TableInfo[P], error) {
	tableNum := atomic.AddInt32(&tempTable, 1)
	tableName := ident.New(fmt.Sprintf("tbl_%d", tableNum))
	table := ident.NewTable(enclosing, tableName)

	err := retry.Execute(ctx, pool, fmt.Sprintf(schemaSpec, table))
	return TableInfo[P]{pool, table}, errors.WithStack(err)
}
