// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package server

import (
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/besteffort"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/core"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/immediate"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/retire"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/scheduler"
	script2 "github.com/cockroachdb/cdc-sink/internal/sequencer/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/switcher"
	"github.com/cockroachdb/cdc-sink/internal/sinkprod"
	"github.com/cockroachdb/cdc-sink/internal/source/cdc"
	"github.com/cockroachdb/cdc-sink/internal/staging"
	"github.com/cockroachdb/cdc-sink/internal/staging/checkpoint"
	"github.com/cockroachdb/cdc-sink/internal/staging/leases"
	"github.com/cockroachdb/cdc-sink/internal/staging/stage"
	"github.com/cockroachdb/cdc-sink/internal/target"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/dlq"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/stdserver"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/google/wire"
	"net"
)

// Injectors from injector.go:

func NewServer(ctx *stopper.Context, config *Config) (*stdserver.Server, error) {
	diagnostics := diag.New(ctx)
	configs, err := applycfg.ProvideConfigs(diagnostics)
	if err != nil {
		return nil, err
	}
	cdcConfig := &config.CDC
	scriptConfig := cdc.ProvideScriptConfig(cdcConfig)
	loader, err := script.ProvideLoader(ctx, configs, scriptConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	eagerConfig, err := ProvideEagerConfig(config, loader)
	if err != nil {
		return nil, err
	}
	stagingConfig := &eagerConfig.Staging
	targetConfig := &eagerConfig.Target
	stagingPool, err := sinkprod.ProvideStagingPool(ctx, stagingConfig, diagnostics, targetConfig)
	if err != nil {
		return nil, err
	}
	stagingSchema, err := sinkprod.ProvideStagingDB(stagingConfig)
	if err != nil {
		return nil, err
	}
	authenticator, err := ProvideAuthenticator(ctx, diagnostics, config, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	listener, err := ProvideListener(ctx, config, diagnostics)
	if err != nil {
		return nil, err
	}
	targetPool, err := sinkprod.ProvideTargetPool(ctx, targetConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	targetStatements, err := sinkprod.ProvideStatementCache(targetConfig, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	dlqConfig := cdc.ProvideDLQConfig(cdcConfig)
	watchers, err := schemawatch.ProvideFactory(ctx, targetPool, diagnostics)
	if err != nil {
		return nil, err
	}
	dlQs := dlq.ProvideDLQs(dlqConfig, targetPool, watchers)
	acceptor, err := apply.ProvideAcceptor(ctx, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, err
	}
	checkpoints, err := checkpoint.ProvideCheckpoints(ctx, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	sequencerConfig := cdc.ProvideSequencerConfig(cdcConfig)
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
	targets, err := cdc.ProvideTargets(ctx, acceptor, cdcConfig, checkpoints, retireRetire, stagingPool, switcherSwitcher, watchers)
	if err != nil {
		return nil, err
	}
	handler := &cdc.Handler{
		Authenticator: authenticator,
		Config:        cdcConfig,
		TargetPool:    targetPool,
		Targets:       targets,
	}
	serveMux := ProvideMux(handler, stagingPool, targetPool)
	tlsConfig, err := ProvideTLSConfig(config)
	if err != nil {
		return nil, err
	}
	server := ProvideServer(ctx, authenticator, diagnostics, listener, serveMux, tlsConfig)
	return server, nil
}

// Injectors from test_fixture.go:

// We want this to be as close as possible to NewServer, it just exposes
// additional plumbing details via the returned testFixture pointer.
func newTestFixture(context *stopper.Context, config *Config) (*testFixture, func(), error) {
	diagnostics := diag.New(context)
	stagingConfig := &config.Staging
	targetConfig := &config.Target
	stagingPool, err := sinkprod.ProvideStagingPool(context, stagingConfig, diagnostics, targetConfig)
	if err != nil {
		return nil, nil, err
	}
	stagingSchema, err := sinkprod.ProvideStagingDB(stagingConfig)
	if err != nil {
		return nil, nil, err
	}
	authenticator, err := ProvideAuthenticator(context, diagnostics, config, stagingPool, stagingSchema)
	if err != nil {
		return nil, nil, err
	}
	listener, err := ProvideListener(context, config, diagnostics)
	if err != nil {
		return nil, nil, err
	}
	cdcConfig := &config.CDC
	targetPool, err := sinkprod.ProvideTargetPool(context, targetConfig, diagnostics)
	if err != nil {
		return nil, nil, err
	}
	targetStatements, err := sinkprod.ProvideStatementCache(targetConfig, targetPool, diagnostics)
	if err != nil {
		return nil, nil, err
	}
	configs, err := applycfg.ProvideConfigs(diagnostics)
	if err != nil {
		return nil, nil, err
	}
	dlqConfig := cdc.ProvideDLQConfig(cdcConfig)
	watchers, err := schemawatch.ProvideFactory(context, targetPool, diagnostics)
	if err != nil {
		return nil, nil, err
	}
	dlQs := dlq.ProvideDLQs(dlqConfig, targetPool, watchers)
	acceptor, err := apply.ProvideAcceptor(context, targetStatements, configs, diagnostics, dlQs, targetPool, watchers)
	if err != nil {
		return nil, nil, err
	}
	checkpoints, err := checkpoint.ProvideCheckpoints(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, nil, err
	}
	sequencerConfig := cdc.ProvideSequencerConfig(cdcConfig)
	stagers := stage.ProvideFactory(stagingPool, stagingSchema, context)
	retireRetire := retire.ProvideRetire(sequencerConfig, stagingPool, stagers)
	typesLeases, err := leases.ProvideLeases(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, nil, err
	}
	schedulerScheduler, err := scheduler.ProvideScheduler(context, sequencerConfig)
	if err != nil {
		return nil, nil, err
	}
	bestEffort := besteffort.ProvideBestEffort(sequencerConfig, typesLeases, schedulerScheduler, stagingPool, stagers, targetPool, watchers)
	coreCore := core.ProvideCore(sequencerConfig, typesLeases, schedulerScheduler, stagers, stagingPool, targetPool)
	immediateImmediate := immediate.ProvideImmediate(targetPool)
	scriptConfig := cdc.ProvideScriptConfig(cdcConfig)
	loader, err := script.ProvideLoader(context, configs, scriptConfig, diagnostics)
	if err != nil {
		return nil, nil, err
	}
	sequencer := script2.ProvideSequencer(loader, targetPool, watchers)
	switcherSwitcher := switcher.ProvideSequencer(bestEffort, coreCore, diagnostics, immediateImmediate, sequencer, stagingPool, targetPool)
	targets, err := cdc.ProvideTargets(context, acceptor, cdcConfig, checkpoints, retireRetire, stagingPool, switcherSwitcher, watchers)
	if err != nil {
		return nil, nil, err
	}
	handler := &cdc.Handler{
		Authenticator: authenticator,
		Config:        cdcConfig,
		TargetPool:    targetPool,
		Targets:       targets,
	}
	serveMux := ProvideMux(handler, stagingPool, targetPool)
	tlsConfig, err := ProvideTLSConfig(config)
	if err != nil {
		return nil, nil, err
	}
	server := ProvideServer(context, authenticator, diagnostics, listener, serveMux, tlsConfig)
	serverTestFixture := &testFixture{
		Authenticator: authenticator,
		Config:        config,
		Diagnostics:   diagnostics,
		Listener:      listener,
		StagingPool:   stagingPool,
		Server:        server,
		StagingDB:     stagingSchema,
		Stagers:       stagers,
		Watcher:       watchers,
	}
	return serverTestFixture, func() {
	}, nil
}

// injector.go:

var completeSet = wire.NewSet(
	Set, cdc.Set, diag.New, retire.Set, script.Set, sinkprod.Set, staging.Set, switcher.Set, target.Set,
)

// test_fixture.go:

type testFixture struct {
	Authenticator types.Authenticator
	Config        *Config
	Diagnostics   *diag.Diagnostics
	Listener      net.Listener
	StagingPool   *types.StagingPool
	Server        *stdserver.Server
	StagingDB     ident.StagingSchema
	Stagers       types.Stagers
	Watcher       types.Watchers
}
