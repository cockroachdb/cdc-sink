// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package script

import (
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/applycfg"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
)

import (
	_ "embed"
)

// Injectors from injector.go:

// Evaluate the loaded script.
func Evaluate(ctx *stopper.Context, loader *Loader, configs *applycfg.Configs, diags *diag.Diagnostics, targetSchema TargetSchema, watchers types.Watchers) (*UserScript, error) {
	userScript, err := ProvideUserScript(configs, loader, diags, targetSchema, watchers)
	if err != nil {
		return nil, err
	}
	return userScript, nil
}

func newScriptFromFixture(fixture *all.Fixture, config *Config, targetSchema TargetSchema) (*UserScript, error) {
	configs := fixture.Configs
	loader, err := ProvideLoader(config)
	if err != nil {
		return nil, err
	}
	diagnostics := fixture.Diagnostics
	watchers := fixture.Watchers
	userScript, err := ProvideUserScript(configs, loader, diagnostics, targetSchema, watchers)
	if err != nil {
		return nil, err
	}
	return userScript, nil
}
