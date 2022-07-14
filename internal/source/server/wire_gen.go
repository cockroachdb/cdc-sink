// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package server

import (
	"context"
	"github.com/cockroachdb/cdc-sink/internal/source/cdc"
	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/target/apply/fan"
	"github.com/cockroachdb/cdc-sink/internal/target/resolve"
	"github.com/cockroachdb/cdc-sink/internal/target/schemawatch"
	"github.com/cockroachdb/cdc-sink/internal/target/sinktest"
	"github.com/cockroachdb/cdc-sink/internal/target/stage"
	"github.com/cockroachdb/cdc-sink/internal/target/timekeeper"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"net"
)

import (
	_ "net/http/pprof"
)

// Injectors from injector.go:

func NewServer(ctx context.Context, config Config) (*Server, func(), error) {
	listener, cleanup, err := ProvideListener(config)
	if err != nil {
		return nil, nil, err
	}
	pool, cleanup2, err := ProvidePool(ctx, config)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	stagingDB := ProvideStagingDB(config)
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
	applyMode := ProvideApplyMode(config)
	metaTable := ProvideMetaTable(stagingDB)
	stagers := stage.ProvideFactory(pool, stagingDB)
	targetTable := ProvideTimeTable(stagingDB)
	timeKeeper, cleanup7, err := timekeeper.ProvideTimeKeeper(ctx, pool, targetTable)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	resolvers, cleanup8, err := resolve.ProvideFactory(ctx, appliers, metaTable, pool, stagers, timeKeeper, watchers)
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
	handler := &cdc.Handler{
		Appliers:      appliers,
		Authenticator: authenticator,
		Mode:          applyMode,
		Pool:          pool,
		Resolvers:     resolvers,
		Stores:        stagers,
	}
	serveMux := ProvideMux(handler, pool, configs)
	tlsConfig, err := ProvideTLSConfig(config)
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
	server, cleanup9 := ProvideServer(listener, serveMux, tlsConfig)
	return server, func() {
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

func newTestFixture(serverShouldUseWebhook shouldUseWebhook, serverShouldUseImmediate shouldUseImmediate) (*testFixture, func(), error) {
	contextContext, cleanup, err := sinktest.ProvideContext()
	if err != nil {
		return nil, nil, err
	}
	dbInfo, err := sinktest.ProvideDBInfo(contextContext)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	pool := sinktest.ProvidePool(dbInfo)
	stagingDB, cleanup2, err := sinktest.ProvideStagingDB(contextContext, pool)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	testDB, cleanup3, err := sinktest.ProvideTestDB(contextContext, pool)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	baseFixture := sinktest.BaseFixture{
		Context:   contextContext,
		DBInfo:    dbInfo,
		Pool:      pool,
		StagingDB: stagingDB,
		TestDB:    testDB,
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
	fans := &fan.Fans{
		Appliers: appliers,
		Pool:     pool,
	}
	metaTable := sinktest.ProvideMetaTable(stagingDB, testDB)
	stagers := stage.ProvideFactory(pool, stagingDB)
	targetTable := sinktest.ProvideTimestampTable(stagingDB, testDB)
	timeKeeper, cleanup7, err := timekeeper.ProvideTimeKeeper(contextContext, pool, targetTable)
	if err != nil {
		cleanup6()
		cleanup5()
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	resolvers, cleanup8, err := resolve.ProvideFactory(contextContext, appliers, metaTable, pool, stagers, timeKeeper, watchers)
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
	watcher, err := sinktest.ProvideWatcher(contextContext, testDB, watchers)
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
	fixture := &sinktest.Fixture{
		BaseFixture: baseFixture,
		Appliers:    appliers,
		Configs:     configs,
		Fans:        fans,
		Resolvers:   resolvers,
		Stagers:     stagers,
		TimeKeeper:  timeKeeper,
		Watchers:    watchers,
		MetaTable:   metaTable,
		Watcher:     watcher,
	}
	serverConnectionMode := provideConnectionMode(dbInfo, serverShouldUseWebhook)
	config := provideTestConfig(serverConnectionMode, serverShouldUseImmediate)
	authenticator, cleanup9, err := ProvideAuthenticator(contextContext, pool, config, stagingDB)
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
	listener, cleanup10, err := ProvideListener(config)
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
	applyMode := ProvideApplyMode(config)
	handler := &cdc.Handler{
		Appliers:      appliers,
		Authenticator: authenticator,
		Mode:          applyMode,
		Pool:          pool,
		Resolvers:     resolvers,
		Stores:        stagers,
	}
	serveMux := ProvideMux(handler, pool, configs)
	tlsConfig, err := ProvideTLSConfig(config)
	if err != nil {
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
		return nil, nil, err
	}
	server, cleanup11 := ProvideServer(listener, serveMux, tlsConfig)
	serverTestFixture := &testFixture{
		Fixture:       fixture,
		Authenticator: authenticator,
		Config:        config,
		Listener:      listener,
		Server:        server,
	}
	return serverTestFixture, func() {
		cleanup11()
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
	*sinktest.Fixture
	Authenticator types.Authenticator
	Config        Config
	Listener      net.Listener
	Server        *Server
}
