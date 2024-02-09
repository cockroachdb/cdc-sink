// Copyright 2024 The Cockroach Authors
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

package sinkprod

import (
	"time"

	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/stdpool"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/spf13/pflag"
)

// StagingConfig defines staging-database connection behaviors.
type StagingConfig struct {
	// Connection string for the target cluster.
	Conn string
	// The maximum lifetime for a database connection; improves
	// loadbalancer compatibility.
	Lifetime time.Duration
	// The number of connections to the target database. If zero, a
	// default value will be used.
	PoolSize int
	// The name of a SQL schema in the staging cluster to store
	// metadata in.
	Schema ident.Schema
}

// Bind adds flags to the set.
func (c *StagingConfig) Bind(f *pflag.FlagSet) {
	f.StringVar(&c.Conn, "stagingConn", "",
		"the staging database's connection string")
	f.IntVar(&c.PoolSize, "stagingDBConns", defaultPoolSize,
		"the maximum pool size to the staging cluster")
	f.DurationVar(&c.Lifetime, "stagingDBLifetime", defaultLifetime,
		"the maximum lifetime for an staging database connection")

	c.Schema = ident.MustSchema(ident.New("_cdc_sink"), ident.Public)
	f.Var(ident.NewSchemaFlag(&c.Schema), "stagingDB",
		"a SQL database schema to store metadata in")
}

// Preflight ensures that unset configuration options have sane defaults
// and returns an error if the StagingConfig is missing any fields for
// which a default cannot be provided.
func (c *StagingConfig) Preflight() error {
	// Staging connection may be empty, since target may be used.
	if c.Lifetime == 0 {
		c.Lifetime = defaultLifetime
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaultPoolSize
	}
	if c.Schema.Empty() {
		c.Schema = ident.MustSchema(ident.New("_cdc_sink"), ident.Public)
	}
	return nil
}

// ProvideStagingDB is called by Wire to retrieve the name of the
// _cdc_sink SQL DATABASE.
func ProvideStagingDB(config *StagingConfig) (ident.StagingSchema, error) {
	return ident.StagingSchema(config.Schema), nil
}

// ProvideStagingPool is called by Wire to create a connection pool that
// accesses the staging cluster. The pool will be closed when the
// context is stopped.
func ProvideStagingPool(
	ctx *stopper.Context, config *StagingConfig, diags *diag.Diagnostics, tgtConfig *TargetConfig,
) (*types.StagingPool, error) {
	// Use target endpoint if needed.
	conn := config.Conn
	if conn == "" {
		conn = tgtConfig.Conn
	}

	ret, err := stdpool.OpenPgxAsStaging(ctx,
		conn,
		stdpool.WithConnectionLifetime(config.Lifetime),
		stdpool.WithDiagnostics(diags, "staging"),
		stdpool.WithMetrics("staging"),
		stdpool.WithPoolSize(config.PoolSize),
		stdpool.WithTransactionTimeout(time.Minute), // Staging shouldn't take that much time.
	)
	if err != nil {
		return nil, err
	}
	ctx.Defer(ret.Close)

	// This sanity-checks the configured schema against the product. For
	// Cockroach and Postgresql, we'll add any missing "public" schema
	// names.
	sch, err := ret.Product.ExpandSchema(config.Schema)
	if err != nil {
		return nil, err
	}
	config.Schema = sch

	return ret, err
}
