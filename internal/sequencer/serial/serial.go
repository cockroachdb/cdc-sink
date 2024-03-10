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

// Package serial contains a sequencer that preserves source
// transactions and relative timestamps.
package serial

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/scheduler"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/sequtil"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/lockset"
	"github.com/cockroachdb/cdc-sink/internal/util/metrics"
	"github.com/cockroachdb/cdc-sink/internal/util/notify"
	"github.com/cockroachdb/cdc-sink/internal/util/retry"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// errPoisoned is a sentinel value used to short-circuit execution.
var errPoisoned = errors.New("poisoned")

// The Serial sequencer accepts batches by writing them to a staging
// table. It will then apply the data in a transactionally-consistent
// and possibly concurrent fashion.
type Serial struct {
	cfg         *sequencer.Config
	leases      types.Leases
	scheduler   *scheduler.Scheduler
	stagers     types.Stagers
	stagingPool *types.StagingPool
	targetPool  *types.TargetPool
}

var _ sequencer.Sequencer = (*Serial)(nil)

// Start implements [sequencer.Sequencer]. It will incrementally unstage
// and apply batches of mutations within the given bounds.
func (s *Serial) Start(
	ctx *stopper.Context, opts *sequencer.StartOptions,
) (types.MultiAcceptor, *notify.Var[sequencer.Stat], error) {
	progress := &notify.Var[sequencer.Stat]{}
	progress.Set(sequencer.NewStat(opts.Group, &ident.TableMap[hlc.Time]{}))

	// Acquire a lease on the group name to prevent multiple sweepers
	// from operating.
	sequtil.LeaseGroup(ctx, s.leases, opts.Group, func(ctx *stopper.Context, group *types.TableGroup) {
		activeGauges := make([]prometheus.Gauge, len(group.Tables))
		for idx, tbl := range group.Tables {
			activeGauges[idx] = sweepActive.WithLabelValues(metrics.TableValues(tbl)...)
		}
		for _, active := range activeGauges {
			active.Set(1)
		}
		defer func() {
			for _, active := range activeGauges {
				active.Set(0)
			}
		}()

		// Open a reader over the staging tables.
		batches, err := s.stagers.Read(ctx, &types.StagingQuery{
			Bounds:      opts.Bounds,
			SegmentSize: s.cfg.FlushSize,
			SweepLimit:  s.cfg.SweepLimit,
			Targets:     group.Tables,
		})
		if err != nil {
			log.WithError(err).Warnf(
				"could not open staging table reader for %s; will retry",
				group)
		}

		// Allow a soft failure mode in best-effort mode.
		poisoned := newPoisonSet(opts.MaxDeferred)

		// A scheduling key to correctly order incremental progress
		// updates versus arriving at the end of a checkpoint window.
		progressKey := []string{fmt.Sprintf("__PROGRESS__:%s", group.Name)}

		metricLabels := metrics.SchemaValues(group.Enclosing)
		template := &serialRound{
			Serial:   s,
			delegate: opts.Delegate,
			group:    group,
			poisoned: poisoned,

			// Metrics.
			applied:     sweepAppliedCount.WithLabelValues(metricLabels...),
			duration:    sweepDuration.WithLabelValues(metricLabels...),
			lastAttempt: sweepLastAttempt.WithLabelValues(metricLabels...),
			lastSuccess: sweepLastSuccess.WithLabelValues(metricLabels...),
			skew:        sweepSkewCount.WithLabelValues(metricLabels...),
		}
		nextRound := func() *serialRound {
			cpy := *template
			return &cpy
		}

		var accumulator atomic.Pointer[serialRound]
		accumulator.Store(nextRound())
		var idleWait []lockset.Outcome

		copier := &sequtil.Copier{
			Config: s.cfg,
			Source: batches,
			// We'll get this callback when the copier does not expect
			// any more data to arrive until the bounds are updated.
			Idle: func(ctx *stopper.Context, bounds hlc.Range) error {
				log.Tracef("idle notification for %s", group)
				// Stop if a one-shot run was requested.
				if opts.OneShot {
					defer func() {
						log.Tracef("Serial.copy: one shot exit: %s", group)
						ctx.Stop(time.Minute)
					}()
				}

				// Ensure that all tasks associated with the round have
				// completed successfully before we report any progress.
				waitFor := idleWait
				idleWait = nil
				if err := lockset.Wait(ctx, waitFor); err != nil {
					log.WithError(err).Tracef("lockset wait error for %s", group)
					return err
				}

				// Stop if too many keys are poisoned.
				if poisoned.HitMax() {
					log.Debugf("Serial.copy: maximum deferrals hit in %s; backing off",
						group)
					return errPoisoned
				}

				// Don't report progress, but keep running if we haven't
				// hit the deferral threshold yet.
				if !poisoned.IsClean() {
					log.Tracef("Serial.copy: poisoned exit %s", group)
					return nil
				}

				advancedTo := bounds.MaxInclusive()
				_, _, _ = progress.Update(func(stat sequencer.Stat) (sequencer.Stat, error) {
					stat = stat.Copy()
					for _, table := range group.Tables {
						stat.Progress().Put(table, advancedTo)
					}
					return stat, nil
				})

				log.Tracef("full progress update: %s to %s (lag %s)",
					group, advancedTo, time.Since(time.Unix(0, advancedTo.Nanos())))
				return nil
			},
			Flush: func(ctx *stopper.Context, batch *types.MultiBatch, segment bool) error {
				// Short-circuit if we've hit too many bad keys.
				if poisoned.HitMax() {
					return errPoisoned
				}

				acc := accumulator.Load()
				if err := acc.accumulate(batch); err != nil {
					return err
				}
				// If it's a partial segment, we'll keep the accumulator
				// into the next update.
				if segment {
					return nil
				}

				// Start a task to store the new data.
				commitOutcome := acc.scheduleCommit(ctx, s.scheduler)
				idleWait = append(idleWait, commitOutcome)

				// Start a task to push progress forward.
				advancedTo := acc.advanceTo
				s.scheduler.Schedule(progressKey, func() error {
					// Don't report progress if we have poisoned timestamps.
					if !poisoned.IsClean() {
						return nil
					}
					if err := lockset.Wait(ctx, []lockset.Outcome{commitOutcome}); err != nil {
						return nil
					}
					log.Tracef("incremental progress update: %s to %s (lag %s)",
						group, advancedTo, time.Since(time.Unix(0, advancedTo.Nanos())))
					_, _, _ = progress.Update(func(stat sequencer.Stat) (sequencer.Stat, error) {
						stat = stat.Copy()
						for _, table := range group.Tables {
							stat.Progress().Put(table, advancedTo)
						}
						return stat, nil
					})
					return nil
				})

				// Store a new accumulator.
				accumulator.Store(nextRound())
				return nil
			},
		}

		// Run copier synchronously.
		if err := copier.Run(ctx); err != nil {
			if errors.Is(err, errPoisoned) {
				// Too many soft FK failures, retry later on.
				log.Debugf("reached soft limit (%d) on FK errors; backing off", opts.MaxDeferred)
			} else if !errors.Is(err, context.Canceled) {
				// Filter out redundant logging.
				log.WithError(err).Warn("error while copying mutations; will continue")
			}
			ctx.Stop(-1) // Fast stop of other tasks.
			return
		}
	})
	return &acceptor{s}, progress, nil
}

type serialRound struct {
	*Serial

	// Initialized by caller.
	delegate types.MultiAcceptor
	group    *types.TableGroup
	poisoned *poisonSet

	// The last time within the accumulated data. This is used for
	// partial progress reporting.
	advanceTo hlc.Time
	// Accumulates (multi-segment) tasks. This represents a potential
	// point of memory exhaustion; we may want to be able to spill this
	// to disk. We need to keep this information available in order to
	// be able to retry in case of 40001, etc. errors.
	batch          *types.MultiBatch
	mutationCount  int
	timestampCount int

	// Metrics
	applied     prometheus.Counter
	duration    prometheus.Observer
	lastAttempt prometheus.Gauge
	lastSuccess prometheus.Gauge
	skew        prometheus.Counter
}

func (r *serialRound) accumulate(segment *types.MultiBatch) error {
	r.mutationCount += segment.Count()
	for _, temp := range segment.Data {
		r.timestampCount++
		if hlc.Compare(temp.Time, r.advanceTo) > 0 {
			r.advanceTo = temp.Time
		}
	}
	if r.batch == nil {
		r.batch = segment
		return nil
	}
	return segment.CopyInto(r.batch)
}

// scheduleCommit handles the error-retry logic around tryCommit.
func (r *serialRound) scheduleCommit(ctx context.Context, s *scheduler.Scheduler) lockset.Outcome {
	var secondChance bool
	return s.Batch(r.batch, func() error {
		// If the batch touches poisoned keys, do nothing.
		if r.poisoned.IsPoisoned(r.batch) {
			return errPoisoned
		}

		// Internally retry 40001, etc. errors.
		err := retry.Retry(ctx, func(ctx context.Context) error {
			return r.tryCommit(ctx)
		})
		if err == nil {
			return nil
		}

		// Give the keys in the batch a second chance to be applied if
		// the initial error is an FK violation.
		if sequtil.IsDeferrableError(err) && !secondChance {
			secondChance = true
			return lockset.RetryAtHead(err)
		}

		// These keys can no longer be operated upon until we restart
		// from the top.
		r.poisoned.MarkPoisoned(r.batch)
		if !errors.Is(err, context.Canceled) {
			log.WithError(err).Warnf("serialRound: could not commit batch in %s", r.group)
		}
		return err
	})
}

// tryCommit attempts to commit the batch. It will send the data to the
// target and mark the mutations as applied within staging.
func (r *serialRound) tryCommit(ctx context.Context) error {
	r.lastAttempt.SetToCurrentTime()
	start := time.Now()
	var err error

	toMark := ident.TableMap[[]types.Mutation]{} // Passed to staging.
	_ = r.batch.CopyInto(types.AccumulatorFunc(func(table ident.Table, mut types.Mutation) error {
		toMark.Put(table, append(toMark.GetZero(table), mut))
		return nil
	}))

	log.Tracef("serialRound.tryCommit: beginning tx for %s to %s", r.group, r.advanceTo)
	targetTx, err := r.targetPool.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = targetTx.Rollback() }()

	if err := r.delegate.AcceptMultiBatch(ctx, r.batch, &types.AcceptOptions{
		TargetQuerier: targetTx,
	}); err != nil {
		return err
	}

	// Close out the transaction, unless we expect
	// additional data in the next callback.
	err = errors.WithStack(targetTx.Commit())
	if err != nil {
		return err
	}

	stagingTx, err := r.stagingPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = stagingTx.Rollback(context.Background()) }()

	// Mark all mutations as applied.
	if err := toMark.Range(func(table ident.Table, muts []types.Mutation) error {
		stager, err := r.stagers.Get(ctx, table)
		if err != nil {
			return err
		}
		return stager.MarkApplied(ctx, stagingTx, muts)
	}); err != nil {
		return err
	}

	// Without X/A transactions, we are in a vulnerable
	// state here. If the staging transaction fails to
	// commit, however, we'd re-apply the work that was just
	// performed. This should wind up being a no-op in the
	// general case.
	err = errors.WithStack(stagingTx.Commit(ctx))
	if err != nil {
		r.skew.Inc()
		return errors.Wrap(err, "serialRound.tryCommit: skew condition")
	}

	log.Tracef("serialRound.tryCommit: commited %s (%d mutations, %d timestamps) to %s",
		r.group, r.mutationCount, r.timestampCount, r.advanceTo)
	r.applied.Add(float64(r.mutationCount))
	r.duration.Observe(time.Since(start).Seconds())
	r.lastSuccess.SetToCurrentTime()

	return nil
}

// poisonSet is a concurrency-safe helper to manage poisoned keys.
type poisonSet struct {
	// maxCount sets a threshold value where all keys are considered
	// poisoned. This enables a graceful backoff and retry in FK schemas
	// to try to get a child table applier loop to "run behind" the
	// applier loop for the parent table.
	maxCount int

	mu struct {
		sync.RWMutex
		data   map[string]struct{}
		hitMax bool
	}
}

func newPoisonSet(maxCount int) *poisonSet {
	ret := &poisonSet{maxCount: maxCount}
	ret.mu.data = make(map[string]struct{}, maxCount)
	return ret
}

// HitMax returns true if the upper bound on the number of poisoned keys
// was hit.
func (p *poisonSet) HitMax() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mu.hitMax
}

// IsClean returns true if no keys were poisoned.
func (p *poisonSet) IsClean() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.mu.hitMax && len(p.mu.data) == 0
}

// IsPoisoned returns true if the maximum number of poisoned keys
// has been hit or if the batch contains any poisoned keys.
func (p *poisonSet) IsPoisoned(batch *types.MultiBatch) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.mu.hitMax {
		return true
	}

	err := batch.CopyInto(types.AccumulatorFunc(func(_ ident.Table, mut types.Mutation) error {
		if _, poisoned := p.mu.data[string(mut.Key)]; poisoned {
			return errPoisoned
		}
		return nil
	}))
	return err != nil
}

// MarkPoisoned records the keys in the batch to prevent any other
// attempts at processing them.
func (p *poisonSet) MarkPoisoned(batch *types.MultiBatch) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mu.hitMax {
		return
	}

	_ = batch.CopyInto(types.AccumulatorFunc(func(_ ident.Table, mut types.Mutation) error {
		p.mu.data[string(mut.Key)] = struct{}{}
		if len(p.mu.data) >= p.maxCount {
			p.mu.hitMax = true
			return errPoisoned
		}
		return nil
	}))
}
