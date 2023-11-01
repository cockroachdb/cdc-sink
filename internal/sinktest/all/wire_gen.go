// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package all

import (
	"github.com/cockroachdb/cdc-sink/internal/sinktest/base"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/staging/stage"
	"github.com/cockroachdb/cdc-sink/internal/staging/version"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/dlq"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/util/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"testing"
)

// Injectors from injector.go:

// NewFixture constructs a self-contained test fixture for all services
// in the target sub-packages.
func NewFixture(t testing.TB) (*Fixture, error) {
	context := base.ProvideContext(t)
	diagnostics := diag.New(context)
	sourcePool, err := base.ProvideSourcePool(context, diagnostics)
	if err != nil {
		return nil, err
	}
	sourceSchema, err := base.ProvideSourceSchema(context, sourcePool)
	if err != nil {
		return nil, err
	}
	stagingPool, err := base.ProvideStagingPool(context)
	if err != nil {
		return nil, err
	}
	stagingSchema, err := base.ProvideStagingSchema(context, stagingPool)
	if err != nil {
		return nil, err
	}
	targetPool, err := base.ProvideTargetPool(context, sourcePool, diagnostics)
	if err != nil {
		return nil, err
	}
	targetStatements := base.ProvideTargetStatements(context, targetPool)
	targetSchema, err := base.ProvideTargetSchema(context, diagnostics, targetPool, targetStatements)
	if err != nil {
		return nil, err
	}
	fixture := &base.Fixture{
		Context:      context,
		SourcePool:   sourcePool,
		SourceSchema: sourceSchema,
		StagingPool:  stagingPool,
		StagingDB:    stagingSchema,
		TargetCache:  targetStatements,
		TargetPool:   targetPool,
		TargetSchema: targetSchema,
	}
	configs, err := applycfg.ProvideConfigs(diagnostics)
	if err != nil {
		return nil, err
	}
	config, err := ProvideDLQConfig()
	if err != nil {
		return nil, err
	}
	watchers, err := schemawatch.ProvideFactory(context, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	dlQs := dlq.ProvideDLQs(config, targetPool, watchers)
	appliers, err := apply.ProvideFactory(context, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	memoMemo, err := memo.ProvideMemo(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	stagers := stage.ProvideFactory(stagingPool, stagingSchema, context)
	checker := version.ProvideChecker(stagingPool, memoMemo)
	watcher, err := ProvideWatcher(targetSchema, watchers)
	if err != nil {
		return nil, err
	}
	allFixture := &Fixture{
		Fixture:        fixture,
		Appliers:       appliers,
		Configs:        configs,
		Diagnostics:    diagnostics,
		DLQConfig:      config,
		DLQs:           dlQs,
		Memo:           memoMemo,
		Stagers:        stagers,
		VersionChecker: checker,
		Watchers:       watchers,
		Watcher:        watcher,
	}
	return allFixture, nil
}
