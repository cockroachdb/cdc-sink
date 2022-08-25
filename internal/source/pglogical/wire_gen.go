// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package pglogical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/apply/fan"
	"github.com/cockroachdb/cdc-sink/internal/target/memo"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
)

// Injectors from injector.go:

// Start creates a PostgreSQL logical replication loop using the
// provided configuration.
func Start(ctx context.Context, config *Config) (*logical.Loop, func(), error) {
	scriptConfig := logical.ProvideUserScriptConfig(config)
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		return nil, nil, err
	}
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		return nil, nil, err
	}
	pool, cleanup, err := logical.ProvidePool(ctx, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingDB, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := apply.ProvideConfigs(ctx, pool, stagingDB)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup3 := schemawatch.ProvideFactory(pool)
	appliers, cleanup4 := apply.ProvideFactory(configs, watchers)
	dialect, err := ProvideDialect(ctx, config)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	fans := &fan.Fans{
		Appliers: appliers,
		Pool:     pool,
	}
	memoMemo, err := memo.ProvideMemo(ctx, pool, stagingDB)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	targetSchema := logical.ProvideUserScriptTarget(baseConfig)
	userScript, err := script.ProvideUserScript(ctx, configs, loader, pool, targetSchema, watchers)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	loop, cleanup5, err := logical.ProvideLoop(ctx, appliers, baseConfig, dialect, fans, memoMemo, pool, userScript)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return loop, func() {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
