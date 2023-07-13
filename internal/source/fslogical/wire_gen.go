// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package fslogical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/staging/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/staging/version"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
)

// Injectors from injector.go:

// Start creates a PostgreSQL logical replication loop using the
// provided configuration.
func Start(contextContext context.Context, config *Config) ([]*logical.Loop, func(), error) {
	scriptConfig, err := logical.ProvideUserScriptConfig(config)
	if err != nil {
		return nil, nil, err
	}
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		return nil, nil, err
	}
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		return nil, nil, err
	}
	stagingPool, cleanup, err := logical.ProvideStagingPool(contextContext, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingDB, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := applycfg.ProvideConfigs(contextContext, stagingPool, stagingDB)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	targetSchema := logical.ProvideUserScriptTarget(baseConfig)
	targetPool, cleanup3, err := logical.ProvideTargetPool(contextContext, baseConfig)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup4 := schemawatch.ProvideFactory(targetPool)
	userScript, err := script.ProvideUserScript(contextContext, configs, loader, stagingPool, targetSchema, watchers)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	client, cleanup5, err := ProvideFirestoreClient(contextContext, config, userScript)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	appliers, cleanup6 := apply.ProvideFactory(configs, watchers)
	memoMemo, err := memo.ProvideMemo(contextContext, stagingPool, stagingDB)
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
	factory, cleanup7, err := logical.ProvideFactory(contextContext, appliers, config, memoMemo, stagingPool, targetPool, userScript, watchers, checker)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	tombstones, err := ProvideTombstones(contextContext, config, client, factory, userScript)
	if err != nil {
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	v, cleanup8, err := ProvideLoops(contextContext, config, client, factory, memoMemo, stagingPool, tombstones, userScript)
	if err != nil {
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return v, func() {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}

// Build remaining testable components from a common fixture.
func startLoopsFromFixture(fixture *all.Fixture, config *Config) ([]*logical.Loop, func(), error) {
	baseFixture := fixture.Fixture
	contextContext := baseFixture.Context
	configs := fixture.Configs
	scriptConfig, err := logical.ProvideUserScriptConfig(config)
	if err != nil {
		return nil, nil, err
	}
	loader, err := script.ProvideLoader(scriptConfig)
	if err != nil {
		return nil, nil, err
	}
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		return nil, nil, err
	}
	stagingPool, cleanup, err := logical.ProvideStagingPool(contextContext, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	targetSchema := logical.ProvideUserScriptTarget(baseConfig)
	watchers := fixture.Watchers
	userScript, err := script.ProvideUserScript(contextContext, configs, loader, stagingPool, targetSchema, watchers)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	client, cleanup2, err := ProvideFirestoreClient(contextContext, config, userScript)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	appliers := fixture.Appliers
	typesMemo := fixture.Memo
	targetPool, cleanup3, err := logical.ProvideTargetPool(contextContext, baseConfig)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	checker := fixture.VersionChecker
	factory, cleanup4, err := logical.ProvideFactory(contextContext, appliers, config, typesMemo, stagingPool, targetPool, userScript, watchers, checker)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	tombstones, err := ProvideTombstones(contextContext, config, client, factory, userScript)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	v, cleanup5, err := ProvideLoops(contextContext, config, client, factory, typesMemo, stagingPool, tombstones, userScript)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return v, func() {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
