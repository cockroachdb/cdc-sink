// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package mylogical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/apply/fan"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
)

// Injectors from injector.go:

// Start creates a MySQL/MariaDB logical replication loop using the
// provided configuration.
func Start(ctx context.Context, config *Config) (*logical.Loop, func(), error) {
	logicalConfig := ProvideBaseConfig(config)
	dialect, err := ProvideDialect(ctx, config)
	if err != nil {
		return nil, nil, err
	}
	pool, cleanup, err := logical.ProvidePool(ctx, logicalConfig)
	if err != nil {
		return nil, nil, err
	}
	watchers, cleanup2 := schemawatch.ProvideFactory(pool)
	appliers, cleanup3 := apply.ProvideFactory(watchers)
	serialPool := logical.ProvideSerializer(logicalConfig, pool)
	querier := logical.ProvideQuerier(pool, serialPool)
	fans := &fan.Fans{
		Appliers: appliers,
		Pool:     querier,
	}
	stagingDB, err := logical.ProvideStagingDB(logicalConfig)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	loop, cleanup4, err := logical.ProvideLoop(ctx, logicalConfig, dialect, fans, pool, serialPool, stagingDB)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return loop, func() {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}