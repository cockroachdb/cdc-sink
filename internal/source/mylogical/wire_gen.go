// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package mylogical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/staging/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/staging/version"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
)

// Injectors from injector.go:

// Start creates a MySQL/MariaDB logical replication loop using the
// provided configuration.
func Start(ctx context.Context, config *Config) (*logical.Loop, func(), error) {
	scriptConfig, err := logical.ProvideUserScriptConfig(config)
	if err != nil {
		return nil, nil, err
	}
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		return nil, nil, err
	}
	dialect, err := ProvideDialect(config, loader)
	if err != nil {
		return nil, nil, err
	}
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		return nil, nil, err
	}
	stagingPool, cleanup, err := logical.ProvideStagingPool(ctx, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingSchema, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := applycfg.ProvideConfigs(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	targetPool, cleanup3, err := logical.ProvideTargetPool(ctx, baseConfig)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup4 := schemawatch.ProvideFactory(targetPool)
	appliers, cleanup5 := apply.ProvideFactory(configs, targetPool, watchers)
	memoMemo, err := memo.ProvideMemo(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	checker := version.ProvideChecker(stagingPool, memoMemo)
	factory, err := logical.ProvideFactory(ctx, appliers, configs, baseConfig, memoMemo, loader, stagingPool, targetPool, watchers, checker)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	loop, cleanup6, err := ProvideLoop(config, dialect, factory)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return loop, func() {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
