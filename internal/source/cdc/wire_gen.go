// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package cdc

import (
	"github.com/cockroachdb/cdc-sink/internal/conveyor"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/besteffort"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/core"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/immediate"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/retire"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/scheduler"
	script2 "github.com/cockroachdb/cdc-sink/internal/sequencer/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/switcher"
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
	"github.com/cockroachdb/cdc-sink/internal/staging/checkpoint"
	"github.com/cockroachdb/cdc-sink/internal/staging/leases"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/dlq"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/util/auth/trust"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
)

// Injectors from test_fixture.go:

func newTestFixture(fixture *all.Fixture, config *Config) (*testFixture, error) {
	sequencerConfig := ProvideSequencerConfig(config)
	baseFixture := fixture.Fixture
	context := baseFixture.Context
	stagingPool := baseFixture.StagingPool
	stagingSchema := baseFixture.StagingDB
	typesLeases, err := leases.ProvideLeases(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	schedulerScheduler, err := scheduler.ProvideScheduler(context, sequencerConfig)
	if err != nil {
		return nil, err
	}
	stagers := fixture.Stagers
	targetPool := baseFixture.TargetPool
	diagnostics := diag.New(context)
	watchers, err := schemawatch.ProvideFactory(context, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	bestEffort := besteffort.ProvideBestEffort(sequencerConfig, typesLeases, schedulerScheduler, stagingPool, stagers, targetPool, watchers)
	authenticator := trust.New()
	targetStatements := baseFixture.TargetCache
	configs := fixture.Configs
	dlqConfig := ProvideDLQConfig(config)
	dlQs := dlq.ProvideDLQs(dlqConfig, targetPool, watchers)
	acceptor, err := apply.ProvideAcceptor(context, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	checkpoints, err := checkpoint.ProvideCheckpoints(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	retireRetire := retire.ProvideRetire(sequencerConfig, stagingPool, stagers)
	coreCore := core.ProvideCore(sequencerConfig, typesLeases, schedulerScheduler, stagers, stagingPool, targetPool)
	immediateImmediate := immediate.ProvideImmediate(targetPool)
	scriptConfig := ProvideScriptConfig(config)
	loader, err := script.ProvideLoader(context, configs, scriptConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	sequencer := script2.ProvideSequencer(loader, targetPool, watchers)
	switcherSwitcher := switcher.ProvideSequencer(bestEffort, coreCore, diagnostics, immediateImmediate, sequencer, stagingPool, targetPool)
	conveyors, err := ProvideConveyors(context, acceptor, config, checkpoints, retireRetire, stagingPool, switcherSwitcher, watchers)
	if err != nil {
		return nil, err
	}
	handler := &Handler{
		Authenticator: authenticator,
		Config:        config,
		TargetPool:    targetPool,
		Conveyors:     conveyors,
	}
	cdcTestFixture := &testFixture{
		Fixture:    fixture,
		BestEffort: bestEffort,
		Handler:    handler,
		Conveyors:  conveyors,
	}
	return cdcTestFixture, nil
}

// test_fixture.go:

type testFixture struct {
	*all.Fixture
	BestEffort *besteffort.BestEffort
	Handler    *Handler
	Conveyors  *conveyor.Conveyors
}
