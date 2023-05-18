// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package script

import (
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
)

import (
	_ "embed"
)

// Injectors from injector.go:

func newScriptFromFixture(fixture *all.Fixture, config *Config, targetSchema TargetSchema) (*UserScript, error) {
	baseFixture := fixture.Fixture
	context := baseFixture.Context
	configs := fixture.Configs
	loader, err := ProvideLoader(config)
	if err != nil {
		return nil, err
	}
	pool := baseFixture.Pool
	watchers := fixture.Watchers
	userScript, err := ProvideUserScript(context, configs, loader, pool, targetSchema, watchers)
	if err != nil {
		return nil, err
	}
	return userScript, nil
}
