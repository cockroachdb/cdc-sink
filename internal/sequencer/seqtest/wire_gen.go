// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package seqtest

import (
	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/besteffort"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/chaos"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/immediate"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/retire"
	script2 "github.com/cockroachdb/cdc-sink/internal/sequencer/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/serial"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/shingle"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/switcher"
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
)

// Injectors from injector.go:

func NewSequencerFixture(fixture *all.Fixture, config *sequencer.Config, scriptConfig *script.Config) (*Fixture, error) {
	baseFixture := fixture.Fixture
	context := baseFixture.Context
	stagingPool := baseFixture.StagingPool
	stagingSchema := baseFixture.StagingDB
	leases, err := provideLeases(context, stagingPool, stagingSchema)
	if err != nil {
		return nil, err
	}
	stagers := fixture.Stagers
	targetPool := baseFixture.TargetPool
	watchers := fixture.Watchers
	bestEffort := besteffort.ProvideBestEffort(config, leases, stagingPool, stagers, targetPool, watchers)
	chaosChaos := &chaos.Chaos{
		Config: config,
	}
	immediateImmediate := &immediate.Immediate{}
	retireRetire := retire.ProvideRetire(config, stagingPool, stagers)
	serialSerial := serial.ProvideSerial(config, leases, stagers, stagingPool, targetPool)
	configs := fixture.Configs
	diagnostics := fixture.Diagnostics
	loader, err := script.ProvideLoader(configs, scriptConfig, diagnostics)
	if err != nil {
		return nil, err
	}
	scriptSequencer := script2.ProvideSequencer(loader, targetPool, watchers)
	shingleShingle := shingle.ProvideShingle(config, stagers, stagingPool, targetPool)
	switcherSwitcher := switcher.ProvideSequencer(bestEffort, diagnostics, immediateImmediate, scriptSequencer, serialSerial, shingleShingle, stagingPool, targetPool)
	seqtestFixture := &Fixture{
		Fixture:    fixture,
		BestEffort: bestEffort,
		Chaos:      chaosChaos,
		Immediate:  immediateImmediate,
		Retire:     retireRetire,
		Serial:     serialSerial,
		Script:     scriptSequencer,
		Shingle:    shingleShingle,
		Switcher:   switcherSwitcher,
	}
	return seqtestFixture, nil
}
