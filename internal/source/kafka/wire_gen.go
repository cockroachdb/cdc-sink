// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package kafka

import (
	"github.com/cockroachdb/field-eng-powertools/stopper"
	"github.com/cockroachdb/replicator/internal/conveyor"
	"github.com/cockroachdb/replicator/internal/script"
	"github.com/cockroachdb/replicator/internal/sequencer/besteffort"
	"github.com/cockroachdb/replicator/internal/sequencer/core"
	"github.com/cockroachdb/replicator/internal/sequencer/immediate"
	"github.com/cockroachdb/replicator/internal/sequencer/retire"
	"github.com/cockroachdb/replicator/internal/sequencer/scheduler"
	script2 "github.com/cockroachdb/replicator/internal/sequencer/script"
	"github.com/cockroachdb/replicator/internal/sequencer/switcher"
	"github.com/cockroachdb/replicator/internal/sinkprod"
	"github.com/cockroachdb/replicator/internal/staging/checkpoint"
	"github.com/cockroachdb/replicator/internal/staging/leases"
	"github.com/cockroachdb/replicator/internal/staging/stage"
	"github.com/cockroachdb/replicator/internal/target/apply"
	"github.com/cockroachdb/replicator/internal/target/dlq"
	"github.com/cockroachdb/replicator/internal/target/schemawatch"
	"github.com/cockroachdb/replicator/internal/util/applycfg"
	"github.com/cockroachdb/replicator/internal/util/diag"
)

// Injectors from injector.go:

// Start creates a Kafka logical replication loop using the
// provided configuration.
func Start(ctx *stopper.Context, config *Config) (*Kafka, error) {
	diagnostics := diag.New(ctx)
	configs, err := applycfg.ProvideConfigs(diagnostics)
	if err != nil {
		return nil, err
	}
	scriptConfig := &config.Script
	loader, err := script.ProvideLoader(ctx, configs, scriptConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	eagerConfig := ProvideEagerConfig(config, loader)
	targetConfig := &eagerConfig.Target
	targetPool, err := sinkprod.ProvideTargetPool(ctx, targetConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	targetStatements, err := sinkprod.ProvideStatementCache(ctx, targetConfig, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	dlqConfig := &eagerConfig.DLQ
	watchers, err := schemawatch.ProvideFactory(ctx, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	dlQs := dlq.ProvideDLQs(dlqConfig, targetPool, watchers)
	acceptor, err := apply.ProvideAcceptor(ctx, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	conveyorConfig := ProvideConveyorConfig(config)
	stagingConfig := &eagerConfig.Staging
	stagingPool, err := sinkprod.ProvideStagingPool(ctx, stagingConfig, diagnostics, targetConfig)
	if err != nil {
		return nil, err
	}
	stagingSchema, err := sinkprod.ProvideStagingDB(ctx, stagingConfig, stagingPool)
	if err != nil {
		return nil, err
	}
	checkpoints, err := checkpoint.ProvideCheckpoints(ctx, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	sequencerConfig := &eagerConfig.Sequencer
	stagers := stage.ProvideFactory(stagingPool, stagingSchema, ctx)
	retireRetire := retire.ProvideRetire(sequencerConfig, stagingPool, stagers)
	typesLeases, err := leases.ProvideLeases(ctx, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	schedulerScheduler, err := scheduler.ProvideScheduler(ctx, sequencerConfig)
	if err != nil {
		return nil, err
	}
	bestEffort := besteffort.ProvideBestEffort(sequencerConfig, typesLeases, schedulerScheduler, stagingPool, stagers, targetPool, watchers)
	coreCore := core.ProvideCore(sequencerConfig, typesLeases, schedulerScheduler, stagers, stagingPool, targetPool)
	immediateImmediate := immediate.ProvideImmediate(targetPool)
	sequencer := script2.ProvideSequencer(loader, targetPool, watchers)
	switcherSwitcher := switcher.ProvideSequencer(bestEffort, coreCore, diagnostics, immediateImmediate, sequencer, stagingPool, targetPool)
	conveyors, err := conveyor.ProvideConveyors(ctx, acceptor, conveyorConfig, checkpoints, retireRetire, switcherSwitcher, watchers)
	if err != nil {
		return nil, err
	}
	conn, err := ProvideConn(ctx, config, conveyors)
	if err != nil {
		return nil, err
	}
	kafka := &Kafka{
		Conn:        conn,
		Diagnostics: diagnostics,
	}
	return kafka, nil
}
