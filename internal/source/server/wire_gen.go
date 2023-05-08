// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package server

import (
	"context"
	"net"

	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/cdc"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/leases"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/target/stage"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "net/http/pprof"
)

// Injectors from injector.go:

func NewServer(ctx context.Context, config *Config) (*Server, func(), error) {
	listener, cleanup, err := ProvideListener(config)
	if err != nil {
		return nil, nil, err
	}
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
	pool, cleanup2, err := logical.ProvidePool(ctx, baseConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	stagingDB, err := logical.ProvideStagingDB(baseConfig)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	configs, cleanup3, err := apply.ProvideConfigs(ctx, pool, stagingDB)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup4 := schemawatch.ProvideFactory(pool)
	appliers, cleanup5 := apply.ProvideFactory(configs, watchers)
	authenticator, cleanup6, err := ProvideAuthenticator(ctx, pool, config, stagingDB)
	if err != nil {
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	cdcConfig := &config.CDC
	typesLeases, err := leases.ProvideLeases(ctx, pool, stagingDB)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	metaTable := cdc.ProvideMetaTable(cdcConfig)
	stagers := stage.ProvideFactory(pool, stagingDB)
	resolvers, cleanup7, err := cdc.ProvideResolvers(ctx, cdcConfig, typesLeases, metaTable, pool, stagers, watchers)
	if err != nil {
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
		Pool:          pool,
		Resolvers:     resolvers,
		Stores:        stagers,
	}
	serveMux := ProvideMux(handler, pool, configs)
	tlsConfig, err := ProvideTLSConfig(config)
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
	server, cleanup8 := ProvideServer(listener, serveMux, tlsConfig)
	return server, func() {
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
	authenticator, cleanup2, err := ProvideAuthenticator(contextContext, pool, config, stagingDB)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	listener, cleanup3, err := ProvideListener(config)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	configs, cleanup4, err := apply.ProvideConfigs(contextContext, pool, stagingDB)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	watchers, cleanup5 := schemawatch.ProvideFactory(pool)
	appliers, cleanup6 := apply.ProvideFactory(configs, watchers)
	cdcConfig := &config.CDC
	typesLeases, err := leases.ProvideLeases(contextContext, pool, stagingDB)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	metaTable := cdc.ProvideMetaTable(cdcConfig)
	stagers := stage.ProvideFactory(pool, stagingDB)
	resolvers, cleanup7, err := cdc.ProvideResolvers(contextContext, cdcConfig, typesLeases, metaTable, pool, stagers, watchers)
	if err != nil {
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
		Pool:          pool,
		Resolvers:     resolvers,
		Stores:        stagers,
	}
	serveMux := ProvideMux(handler, pool, configs)
	tlsConfig, err := ProvideTLSConfig(config)
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
	server, cleanup8 := ProvideServer(listener, serveMux, tlsConfig)
	serverTestFixture := &testFixture{
		Authenticator: authenticator,
		Config:        config,
		Listener:      listener,
		Pool:          pool,
		Server:        server,
		StagingDB:     stagingDB,
		Watcher:       watchers,
	}
	return serverTestFixture, func() {
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
	Listener      net.Listener
	Pool          *pgxpool.Pool
	Server        *Server
	StagingDB     ident.StagingDB
	Watcher       types.Watchers
}
