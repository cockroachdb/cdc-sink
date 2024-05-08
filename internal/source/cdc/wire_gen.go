// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package cdc

import (
	"github.com/cockroachdb/replicator/internal/conveyor"
	"github.com/cockroachdb/replicator/internal/script"
	"github.com/cockroachdb/replicator/internal/sequencer/besteffort"
	"github.com/cockroachdb/replicator/internal/sequencer/core"
	"github.com/cockroachdb/replicator/internal/sequencer/immediate"
	"github.com/cockroachdb/replicator/internal/sequencer/retire"
	"github.com/cockroachdb/replicator/internal/sequencer/scheduler"
	script2 "github.com/cockroachdb/replicator/internal/sequencer/script"
	"github.com/cockroachdb/replicator/internal/sequencer/switcher"
	"github.com/cockroachdb/replicator/internal/sinktest/all"
	"github.com/cockroachdb/replicator/internal/staging/checkpoint"
	"github.com/cockroachdb/replicator/internal/staging/leases"
	"github.com/cockroachdb/replicator/internal/target/apply"
	"github.com/cockroachdb/replicator/internal/target/dlq"
	"github.com/cockroachdb/replicator/internal/target/schemawatch"
	"github.com/cockroachdb/replicator/internal/util/auth/trust"
	"github.com/cockroachdb/replicator/internal/util/diag"
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
	targetStatements := baseFixture.TargetCache
	configs := fixture.Configs
	dlqConfig := ProvideDLQConfig(config)
	dlQs := dlq.ProvideDLQs(dlqConfig, targetPool, watchers)
	acceptor, err := apply.ProvideAcceptor(context, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	conveyorConfig := ProvideConveyorConfig(config)
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
	conveyors, err := conveyor.ProvideConveyors(context, acceptor, conveyorConfig, checkpoints, retireRetire, switcherSwitcher, watchers)
	if err != nil {
		return nil, err
	}
	authenticator := trust.New()
	handler, err := ProvideHandler(authenticator, config, conveyors, targetPool)
	if err != nil {
		return nil, err
	}
	cdcTestFixture := &testFixture{
		Fixture:    fixture,
		BestEffort: bestEffort,
		Conveyors:  conveyors,
		Handler:    handler,
	}
	return cdcTestFixture, nil
}

// test_fixture.go:

type testFixture struct {
	*all.Fixture
	BestEffort *besteffort.BestEffort
	Conveyors  *conveyor.Conveyors
	Handler    *Handler
}
