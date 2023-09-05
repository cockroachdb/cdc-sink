// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package server

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/cdc"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/staging/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/staging/leases"
	"github.com/cockroachdb/cdc-sink/internal/staging/memo"
	"github.com/cockroachdb/cdc-sink/internal/staging/stage"
	"github.com/cockroachdb/cdc-sink/internal/staging/version"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"net"
)

// Injectors from injector.go:

func NewServer(ctx context.Context, config *Config) (*Server, func(), error) {
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
	authenticator, cleanup3, err := ProvideAuthenticator(ctx, diagnostics, config, stagingPool, stagingSchema)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	listener, cleanup4, err := ProvideListener(config, diagnostics)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	configs, cleanup5, err := applycfg.ProvideConfigs(ctx, diagnostics, stagingPool, stagingSchema)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	targetPool, cleanup6, err := logical.ProvideTargetPool(ctx, baseConfig, diagnostics)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup7, err := schemawatch.ProvideFactory(targetPool, diagnostics)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	appliers, cleanup8, err := apply.ProvideFactory(configs, diagnostics, targetPool, watchers)
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
	cdcConfig := &config.CDC
	typesLeases, err := leases.ProvideLeases(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	memoMemo, err := memo.ProvideMemo(ctx, stagingPool, stagingSchema)
	if err != nil {
		cleanup8()
		cleanup7()
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
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	metaTable := cdc.ProvideMetaTable(cdcConfig)
	stagers := stage.ProvideFactory(stagingPool, stagingSchema)
	resolvers, cleanup9, err := cdc.ProvideResolvers(ctx, cdcConfig, typesLeases, factory, metaTable, stagingPool, stagers, watchers)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	handler := &cdc.Handler{
		Appliers:      appliers,
		Authenticator: authenticator,
		Config:        cdcConfig,
		Resolvers:     resolvers,
		StagingPool:   stagingPool,
		Stores:        stagers,
		TargetPool:    targetPool,
	}
	serveMux := ProvideMux(handler, stagingPool, targetPool)
	tlsConfig, err := ProvideTLSConfig(config)
	if err != nil {
		cleanup9()
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	server, cleanup10 := ProvideServer(authenticator, diagnostics, listener, serveMux, tlsConfig)
	return server, func() {
		cleanup10()
		cleanup9()
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

// Injectors from test_fixture.go:

// We want this to be as close as possible to Start, it just exposes
// additional plumbing details via the returned testFixture pointer.
func newTestFixture(contextContext context.Context, config *Config) (*testFixture, func(), error) {
	diagnostics, cleanup := diag.New(contextContext)
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
	baseConfig, err := logical.ProvideBaseConfig(config, loader)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	stagingPool, cleanup2, err := logical.ProvideStagingPool(contextContext, baseConfig, diagnostics)
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
	authenticator, cleanup3, err := ProvideAuthenticator(contextContext, diagnostics, config, stagingPool, stagingSchema)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	listener, cleanup4, err := ProvideListener(config, diagnostics)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	configs, cleanup5, err := applycfg.ProvideConfigs(contextContext, diagnostics, stagingPool, stagingSchema)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	targetPool, cleanup6, err := logical.ProvideTargetPool(contextContext, baseConfig, diagnostics)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup7, err := schemawatch.ProvideFactory(targetPool, diagnostics)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	appliers, cleanup8, err := apply.ProvideFactory(configs, diagnostics, targetPool, watchers)
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
	cdcConfig := &config.CDC
	typesLeases, err := leases.ProvideLeases(contextContext, stagingPool, stagingSchema)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	memoMemo, err := memo.ProvideMemo(contextContext, stagingPool, stagingSchema)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	checker := version.ProvideChecker(stagingPool, memoMemo)
	factory, err := logical.ProvideFactory(contextContext, appliers, configs, baseConfig, diagnostics, memoMemo, loader, stagingPool, targetPool, watchers, checker)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	metaTable := cdc.ProvideMetaTable(cdcConfig)
	stagers := stage.ProvideFactory(stagingPool, stagingSchema)
	resolvers, cleanup9, err := cdc.ProvideResolvers(contextContext, cdcConfig, typesLeases, factory, metaTable, stagingPool, stagers, watchers)
	if err != nil {
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	handler := &cdc.Handler{
		Appliers:      appliers,
		Authenticator: authenticator,
		Config:        cdcConfig,
		Resolvers:     resolvers,
		StagingPool:   stagingPool,
		Stores:        stagers,
		TargetPool:    targetPool,
	}
	serveMux := ProvideMux(handler, stagingPool, targetPool)
	tlsConfig, err := ProvideTLSConfig(config)
	if err != nil {
		cleanup9()
		cleanup8()
		cleanup7()
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	server, cleanup10 := ProvideServer(authenticator, diagnostics, listener, serveMux, tlsConfig)
	serverTestFixture := &testFixture{
		Authenticator: authenticator,
		Config:        config,
		Diagnostics:   diagnostics,
		Listener:      listener,
		StagingPool:   stagingPool,
		Server:        server,
		StagingDB:     stagingSchema,
		Watcher:       watchers,
	}
	return serverTestFixture, func() {
		cleanup10()
		cleanup9()
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

// test_fixture.go:

type testFixture struct {
	Authenticator types.Authenticator
	Config        *Config
	Diagnostics   *diag.Diagnostics
	Listener      net.Listener
	StagingPool   *types.StagingPool
	Server        *Server
	StagingDB     ident.StagingSchema
	Watcher       types.Watchers
}
