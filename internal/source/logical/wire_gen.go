// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package logical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
)

// Injectors from injector.go:

func Start(ctx context.Context, config Config, dialect Dialect) (*Loop, func(), error) {
	scriptConfig, err := ProvideUserScriptConfig(config)
	if err != nil {
		return nil, nil, err
	}
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		return nil, nil, err
	}
	baseConfig, err := ProvideBaseConfig(config, loader)
	if err != nil {
		return nil, nil, err
	}
	targetPool, cleanup, err := ProvideTargetPool(ctx, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingDB, err := ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := apply.ProvideConfigs(ctx, targetPool, stagingDB)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup3 := schemawatch.ProvideFactory(targetPool)
	appliers, cleanup4 := apply.ProvideFactory(configs, watchers)
	stagingPool := ProvideStagingPool(targetPool)
	memoMemo, err := memo.ProvideMemo(ctx, stagingPool, stagingDB)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	targetSchema := ProvideUserScriptTarget(baseConfig)
	userScript, err := script.ProvideUserScript(ctx, configs, loader, stagingPool, targetSchema, watchers)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	factory, cleanup5 := ProvideFactory(appliers, config, memoMemo, stagingPool, targetPool, watchers, userScript)
	logicalLoop, err := ProvideLoop(ctx, factory, dialect)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return logicalLoop, func() {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
