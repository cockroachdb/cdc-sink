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

package switcher_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/chaos"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/seqtest"
	"github.com/cockroachdb/cdc-sink/internal/sequencer/switcher"
	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
	"github.com/cockroachdb/cdc-sink/internal/sinktest/recorder"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/notify"
	"github.com/stretchr/testify/require"
)

func TestSwitcherSmoke(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		testSwitcherSmoke(t, false)
	})
	t.Run("chaos", func(t *testing.T) {
		testSwitcherSmoke(t, true)
	})
}

func testSwitcherSmoke(t *testing.T, addChaos bool) {
	t.Helper()
	r := require.New(t)

	fixture, err := all.NewFixture(t)
	r.NoError(err)
	ctx := fixture.Context

	cfg := &sequencer.Config{
		Parallelism:     2,
		QuiescentPeriod: 100 * time.Millisecond,
		SweepLimit:      sequencer.DefaultSweepLimit,
		TimestampLimit:  1,
	}
	if addChaos {
		cfg.Chaos = 0.01
	}
	seqFixture, err := seqtest.NewSequencerFixture(fixture, cfg, &script.Config{})
	r.NoError(err)

	// Create parent and child tables.
	parentInfo, err := fixture.CreateTargetTable(ctx, "CREATE TABLE %s (parent INT PRIMARY KEY)")
	r.NoError(err)

	childInfo, err := fixture.CreateTargetTable(ctx, fmt.Sprintf(
		`CREATE TABLE %%s (
child INT PRIMARY KEY,
parent INT NOT NULL REFERENCES %s,
val INT DEFAULT 0 NOT NULL
)`, parentInfo.Name()))
	r.NoError(err)

	sw := seqFixture.Switcher

	// Write data to the database, but also record method calls.
	rec := &recorder.Recorder{
		Next: types.OrderedAcceptorFrom(fixture.ApplyAcceptor, fixture.Watchers),
	}

	// Control points for the sequencer.
	bounds := &notify.Var[hlc.Range]{}
	group := &types.TableGroup{
		Name:   ident.New(fixture.StagingDB.Raw()),
		Tables: []ident.Table{parentInfo.Name(), childInfo.Name()},
	}
	mode := &notify.Var[switcher.Mode]{}

	mode.Set(switcher.ModeBestEffort) // Mode must be set before starting.
	opts := &sequencer.StartOptions{
		Bounds:   bounds,
		Delegate: rec,
		Group:    group,
	}
	acc, stat, err := sw.Start(ctx, opts, mode)
	r.NoError(err)

	// Tracking variables for generating a reasonable workload.
	batchCounter := 0
	expectedMutations := 0
	hlcTime := int64(0)
	parents := make(map[int]struct{})
	children := make(map[int]struct{})

	// Generate some number of batches.
	sendBatches := func() error {
		for i := int64(0); i < 100; i++ {
			batch := seqtest.GenerateBatch(
				&batchCounter, hlc.New(hlcTime, 0),
				parents, children,
				parentInfo.Name(), childInfo.Name())
			expectedMutations += batch.Count()
		retry:
			if err := acc.AcceptMultiBatch(ctx, batch, &types.AcceptOptions{}); err != nil {
				if errors.Is(err, chaos.ErrChaos) {
					goto retry
				}
				return err
			}

			// Incrementally advance the bounds. Since the max value is
			// exclusive, we need to add 1.
			bounds.Set(hlc.Range{hlc.Zero(), hlc.New(hlcTime, 1)})
			hlcTime++
		}
		return nil
	}
	// Wait until all tables have advanced to the "current" HLC time.
	waitForCatchUp := func(r *require.Assertions) {
		for {
			expected := hlc.New(hlcTime-1, 1) // The hlcTime is ++'ed at the end of sendBatches()
			nextStat, changed := stat.Get()
			caughtUp := hlc.Compare(sequencer.CommonMin(nextStat), expected) >= 0
			if !caughtUp {
				select {
				case <-changed:
					continue
				case <-ctx.Done():
					r.NoError(ctx.Err())
				}
			}

			// Verify that all staged mutations are marked as applied.
			for _, info := range []interface{ Name() ident.Table }{parentInfo, childInfo} {
				muts, err := fixture.PeekStaged(ctx, info.Name(), hlc.Zero(), expected)
				r.NoError(err, info.Name().Raw())
				r.Empty(muts, info.Name().Raw())
			}

			// Ensure that the diagnostic data is sane.
			var diag strings.Builder
			r.NoError(fixture.Diagnostics.Write(ctx, &diag, true))
			return
		}
	}

	// Test toggling modes having waited for the previous to finish.
	t.Run("basic-transitions", func(t *testing.T) {
		r := require.New(t)

		for i := 0; i < 5; i++ {
			mode.Set(switcher.ModeBestEffort)
			r.NoError(sendBatches())
			waitForCatchUp(r)
			if addChaos {
				r.GreaterOrEqual(rec.Count(), expectedMutations)
			} else {
				r.Equal(expectedMutations, rec.Count())
			}

			mode.Set(switcher.ModeSerial)
			r.NoError(sendBatches())
			waitForCatchUp(r)
			if addChaos {
				r.GreaterOrEqual(rec.Count(), expectedMutations)
			} else {
				r.Equal(expectedMutations, rec.Count())
			}

			mode.Set(switcher.ModeShingle)
			r.NoError(sendBatches())
			waitForCatchUp(r)
			if addChaos {
				r.GreaterOrEqual(rec.Count(), expectedMutations)
			} else {
				r.Equal(expectedMutations, rec.Count())
			}
		}
	})

	// This will allow each mode to perform some work and then
	// immediately toggle to the next mode.
	t.Run("jagged-transitions", func(t *testing.T) {
		r := require.New(t)

		_, progressMade := stat.Get()

		for i := 0; i < 5; i++ {
			for _, next := range []switcher.Mode{
				switcher.ModeBestEffort,
				switcher.ModeSerial,
				switcher.ModeShingle,
			} {
				mode.Set(next)
				r.NoError(sendBatches())
				select {
				case <-progressMade:
					_, progressMade = stat.Get()
				case <-ctx.Done():
					r.NoError(ctx.Err())
				}
			}
		}

		waitForCatchUp(r)
		if addChaos {
			r.GreaterOrEqual(rec.Count(), expectedMutations)
		} else {
			r.Equal(expectedMutations, rec.Count())
		}
	})
}
