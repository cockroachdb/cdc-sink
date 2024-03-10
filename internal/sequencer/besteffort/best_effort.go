// Copyright 2024 The Cockroach Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package besteffort contains a best-effort implementation of [types.MultiAcceptor].
package besteffort

import (
	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/scheduler"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/sequtil"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/notify"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/cockroachdb/cdc-sink/internal/util/stopvar"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// BestEffort injects an acceptor shim that will attempt to write
// directly to the target table. It will also run a concurrent
// sequencer for each table that is provided.
type BestEffort struct {
	cfg         *sequencer.Config
	delegate    sequencer.Sequencer
	leases      types.Leases
	scheduler   *scheduler.Scheduler
	stagingPool *types.StagingPool
	stagers     types.Stagers
	targetPool  *types.TargetPool
	timeSource  func() hlc.Time
	watchers    types.Watchers
}

var (
	_ sequencer.Sequencer = (*BestEffort)(nil)
	_ sequencer.Shim      = (*BestEffort)(nil)
)

// SetTimeSource is called by tests that need to ensure lock-step
// behaviors in sweepTable or when testing the proactive timestamp
// behavior.
func (s *BestEffort) SetTimeSource(source func() hlc.Time) {
	s.timeSource = source
}

// Wrap implements [sequencer.Shim] and returns a copy of the receiver.
func (s *BestEffort) Wrap(
	_ *stopper.Context, delegate sequencer.Sequencer,
) (sequencer.Sequencer, error) {
	next := *s
	next.delegate = delegate
	return &next, nil
}

// Start implements [sequencer.Starter]. It will launch a background
// goroutine to attempt to apply staged mutations for each table within
// the group.
func (s *BestEffort) Start(
	ctx *stopper.Context, opts *sequencer.StartOptions,
) (types.MultiAcceptor, *notify.Var[sequencer.Stat], error) {
	if s.delegate == nil {
		return nil, nil, errors.New("call Wrap first")
	}
	stats := &notify.Var[sequencer.Stat]{}
	stats.Set(sequencer.NewStat(opts.Group, &ident.TableMap[hlc.Time]{}))

	// XXX TODO: Opportunistic sweeping

	sequtil.LeaseGroup(ctx, s.leases, opts.Group, func(ctx *stopper.Context, group *types.TableGroup) {
		for _, table := range opts.Group.Tables {
			table := table // Capture.

			// Create an options configuration that executes the
			// delegate Sequencer as though it were configured only as a
			// single table. The group name is changed to ensure that
			// each Start call can acquire its own lease.
			subOpts := opts.Copy()
			subOpts.MaxDeferred = s.cfg.SweepLimit
			subOpts.OneShot = true
			subOpts.Group.Name = ident.New(subOpts.Group.Name.Raw() + ":" + table.Raw())
			subOpts.Group.Tables = []ident.Table{table}

			_, subStats, err := s.delegate.Start(ctx, subOpts)
			if err != nil {
				log.WithError(err).Warnf(
					"BestEffort.Start: could not start nested Sequencer for %s", table)
				return
			}

			// Start a helper to aggregate the progress values together.
			ctx.Go(func() error {
				// Ignoring error since innermost callback returns nil.
				_, _ = stopvar.DoWhenChanged(ctx, nil, subStats, func(ctx *stopper.Context, _, subStat sequencer.Stat) error {
					_, _, err := stats.Update(func(old sequencer.Stat) (sequencer.Stat, error) {
						nextProgress, ok := subStat.Progress().Get(table)
						if !ok {
							return nil, notify.ErrNoUpdate
						}
						next := old.Copy()
						next.Progress().Put(table, nextProgress)
						return next, nil
					})
					return err
				})
				return nil
			})
		}

		// Wait until shutdown.
		<-ctx.Stopping()
	})

	// Respect table-dependency ordering.
	acc := types.OrderedAcceptorFrom(&acceptor{s, opts.Delegate}, s.watchers)

	return acc, stats, nil
}
