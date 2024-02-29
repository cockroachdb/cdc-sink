// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package mylogical

import (
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/chaos"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/immediate"
	script2 "github.com/cockroachdb/cdc-sink/internal/sequencer/script"
	"github.com/cockroachdb/cdc-sink/internal/sinkprod"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/dlq"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/util/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
)

// Injectors from injector.go:

// Start creates a MySQL/MariaDB logical replication loop using the
// provided configuration.
func Start(ctx *stopper.Context, config *Config) (*MYLogical, error) {
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
	targetStatements, err := sinkprod.ProvideStatementCache(targetConfig, targetPool, diagnostics)
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
	sequencerConfig := &eagerConfig.Sequencer
	chaosChaos := &chaos.Chaos{
		Config: sequencerConfig,
	}
	immediateImmediate := &immediate.Immediate{}
	stagingConfig := &eagerConfig.Staging
	stagingPool, err := sinkprod.ProvideStagingPool(ctx, stagingConfig, diagnostics, targetConfig)
	if err != nil {
		return nil, err
	}
	stagingSchema, err := sinkprod.ProvideStagingDB(stagingConfig)
	if err != nil {
		return nil, err
	}
	memoMemo, err := memo.ProvideMemo(ctx, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	sequencer := script2.ProvideSequencer(loader, targetPool, watchers)
	mylogicalConn, err := ProvideConn(ctx, acceptor, chaosChaos, config, immediateImmediate, memoMemo, sequencer, stagingPool, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	myLogical := &MYLogical{
		Conn:        mylogicalConn,
		Diagnostics: diagnostics,
	}
	return myLogical, nil
}
