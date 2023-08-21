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
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
)

// Injectors from injector.go:

// Start creates a MySQL/MariaDB logical replication loop using the
// provided configuration.
func Start(ctx context.Context, config *Config) (*MYLogical, func(), error) {
	diagnostics, cleanup := diag.New(ctx)
	scriptConfig, err := logical.ProvideUserScriptConfig(config)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	dialect, err := ProvideDialect(config, loader)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	stagingPool, cleanup2, err := logical.ProvideStagingPool(ctx, baseConfig, diagnostics)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	stagingSchema, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	configs, cleanup3, err := applycfg.ProvideConfigs(ctx, diagnostics, stagingPool, stagingSchema)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	targetPool, cleanup4, err := logical.ProvideTargetPool(ctx, baseConfig, diagnostics)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup5, err := schemawatch.ProvideFactory(targetPool, diagnostics)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	appliers, cleanup6, err := apply.ProvideFactory(configs, diagnostics, targetPool, watchers)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	memoMemo, err := memo.ProvideMemo(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	checker := version.ProvideChecker(stagingPool, memoMemo)
	factory, err := logical.ProvideFactory(ctx, appliers, configs, baseConfig, diagnostics, memoMemo, loader, stagingPool, targetPool, watchers, checker)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	loop, cleanup7, err := ProvideLoop(config, dialect, factory)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	myLogical := &MYLogical{
		Diagnostics: diagnostics,
		Loop:        loop,
	}
	return myLogical, func() {
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}

// injector.go:

// MYLogical isa MySQL/MariaDB logical replication loop.
type MYLogical struct {
	Diagnostics *diag.Diagnostics
	Loop        *logical.Loop
}
