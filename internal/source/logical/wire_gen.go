// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package logical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/staging/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/staging/version"
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
	stagingPool, cleanup, err := ProvideStagingPool(ctx, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingSchema, err := ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := applycfg.ProvideConfigs(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	targetPool, cleanup3, err := ProvideTargetPool(ctx, baseConfig)
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
	targetSchema := ProvideUserScriptTarget(baseConfig)
	userScript, err := script.ProvideUserScript(ctx, configs, loader, stagingPool, targetSchema, watchers)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	checker := version.ProvideChecker(stagingPool, memoMemo)
	factory, cleanup6, err := ProvideFactory(ctx, appliers, config, memoMemo, stagingPool, targetPool, userScript, watchers, checker)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	logicalLoop, err := ProvideLoop(ctx, factory, dialect)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return logicalLoop, func() {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
