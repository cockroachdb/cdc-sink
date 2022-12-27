// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package cdc

import (
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/source/logical"
	"github.com/cockroachdb/cdc-sink/internal/target/auth/trust"
	"github.com/cockroachdb/cdc-sink/internal/target/sinktest"
)

// Injectors from test_fixture.go:

func newTestFixture(fixture *sinktest.Fixture, config *Config) (*testFixture, func(), error) {
	appliers := fixture.Appliers
	authenticator := trust.New()
	baseFixture := &fixture.BaseFixture
	context := baseFixture.Context
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
	pool, cleanup, err := logical.ProvidePool(context, baseConfig)
	if err != nil {
		return nil, nil, err
	}
	metaTable := ProvideMetaTable(config)
	stagers := fixture.Stagers
	watchers := fixture.Watchers
	resolvers, cleanup2, err := ProvideResolvers(context, config, metaTable, pool, stagers, watchers)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	handler := &Handler{
		Appliers:      appliers,
		Authenticator: authenticator,
		Config:        config,
		Pool:          pool,
		Resolvers:     resolvers,
		Stores:        stagers,
	}
	cdcTestFixture := &testFixture{
		Fixture:   fixture,
		Handler:   handler,
		Resolvers: resolvers,
	}
	return cdcTestFixture, func() {
		cleanup2()
		cleanup()
	}, nil
}

// test_fixture.go:

type testFixture struct {
	*sinktest.Fixture
	Handler   *Handler
	Resolvers *Resolvers
}
