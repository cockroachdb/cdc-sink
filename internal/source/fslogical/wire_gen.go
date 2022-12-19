// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package fslogical

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/memo"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/target/sinktest"
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
	pool, cleanup, err := logical.ProvidePool(contextContext, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingDB, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	configs, cleanup2, err := apply.ProvideConfigs(contextContext, pool, stagingDB)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	targetSchema := logical.ProvideUserScriptTarget(baseConfig)
	watchers, cleanup3 := schemawatch.ProvideFactory(pool)
	userScript, err := script.ProvideUserScript(contextContext, configs, loader, pool, targetSchema, watchers)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	client, cleanup4, err := ProvideFirestoreClient(contextContext, config, userScript)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	appliers, cleanup5 := apply.ProvideFactory(configs, watchers)
	memoMemo, err := memo.ProvideMemo(contextContext, pool, stagingDB)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	factory, cleanup6 := logical.ProvideFactory(appliers, config, memoMemo, pool, watchers, userScript)
	tombstones, err := ProvideTombstones(contextContext, config, client, factory, userScript)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	v, cleanup7, err := ProvideLoops(contextContext, config, client, factory, memoMemo, pool, tombstones, userScript)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return v, func() {
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
func startLoopsFromFixture(fixture *sinktest.Fixture, config *Config) ([]*logical.Loop, func(), error) {
	baseFixture := &fixture.BaseFixture
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
	pool, cleanup, err := logical.ProvidePool(contextContext, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	targetSchema := logical.ProvideUserScriptTarget(baseConfig)
	watchers := fixture.Watchers
	userScript, err := script.ProvideUserScript(contextContext, configs, loader, pool, targetSchema, watchers)
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
	factory, cleanup3 := logical.ProvideFactory(appliers, config, typesMemo, pool, watchers, userScript)
	tombstones, err := ProvideTombstones(contextContext, config, client, factory, userScript)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	v, cleanup4, err := ProvideLoops(contextContext, config, client, factory, typesMemo, pool, tombstones, userScript)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return v, func() {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
