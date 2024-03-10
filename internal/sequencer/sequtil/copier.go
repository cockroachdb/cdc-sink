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

package sequtil

import (
	"time"

	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
)

// A step represents a single iteration of the copy loop.
type step struct {
	idle      *hlc.Range        // Non-nil if no data is expected without updated bounds.
	segmented bool              // Indicates a non-terminal segment of a multipart payload.
	toFlush   *types.MultiBatch // Data to send downstream.
}

// EachFn is a callback from a [Copier].
type EachFn func(ctx *stopper.Context, batch *types.TemporalBatch, segment bool) error

// FlushFn is a callback from a [Copier].
type FlushFn func(ctx *stopper.Context, batch *types.MultiBatch, segment bool) error

// IdleFn is a callback from a [Copier].
type IdleFn func(ctx *stopper.Context, bounds hlc.Range) error

// A Copier consumes a channel of [types.StagingCursor], assembles the
// individual temporal batches into large batches, and invokes event
// callbacks to process the larger batches.
type Copier struct {
	Config *sequencer.Config           // Controls for flush behavior.
	Each   EachFn                      // Optional callback to receive each batch.
	Flush  FlushFn                     // Optional callback to receive aggregated data.
	Idle   IdleFn                      // Optional callback when the source has become idle.
	Source <-chan *types.StagingCursor // Input data.

	accumulator *types.MultiBatch
	count       int
}

// Run copies data from the source to the target. It is a blocking call
// that will return if the context is stopped, the channel is closed, or
// if the target returns an error.
func (c *Copier) Run(ctx *stopper.Context) error {
	for {
		step, err := c.nextStep(ctx)
		if err != nil {
			return err
		}
		// Being stopped.
		if step == nil {
			return nil
		}
		if step.toFlush != nil && c.Flush != nil {
			if err := c.Flush(ctx, step.toFlush, step.segmented); err != nil {
				return err
			}
		}
		// Suppress duplicate idle updates if the bounds change without
		// generating additional updates.
		if step.idle != nil && c.Idle != nil {
			if err := c.Idle(ctx, *step.idle); err != nil {
				return err
			}
		}
	}
}

// nextStep will read from the source and decide what the next step
// should be. A nil step will be returned if the context is being
// stopped.
func (c *Copier) nextStep(ctx *stopper.Context) (*step, error) {
	// Ensure initial state, post-flush.
	if c.accumulator == nil {
		c.accumulator = &types.MultiBatch{}
		c.count = 0
	}

	// Receiving from a nil channel blocks forever. We'll start a flush
	// timer only if there's already some data in the accumulator.
	var timerC <-chan time.Time
	if c.count > 0 {
		timer := time.NewTimer(c.Config.FlushPeriod)
		defer timer.Stop()
		timerC = timer.C
	}

	select {
	case cursor, open := <-c.Source:
		// The channel has been closed. This will happen when the
		// stopper is being stopped.
		if !open {
			return nil, nil
		}
		// There was an error reading from staging.
		if cursor.Error != nil {
			return nil, cursor.Error
		}

		// No more data can be expected from the source, until the read
		// bounds are updated. Flush and return the now-completed
		// timestamp range.
		if !cursor.Idle.Empty() {
			ret := &step{idle: &cursor.Idle}
			if c.count > 0 {
				ret.toFlush = c.accumulator
				c.accumulator = nil
			}
			return ret, nil
		}

		batch := cursor.Batch

		// Allow spying on data as it arrives.
		if c.Each != nil {
			if err := c.Each(ctx, batch, cursor.Segmented); err != nil {
				return nil, err
			}
		}

		// We have an over-large value. Flush immediately and let
		// the caller decide if it's going to open a transaction or
		// send as-is.
		if cursor.Segmented {
			ret := &step{toFlush: c.accumulator, segmented: true}
			// Rather than sometimes returning a slice of batches, we'll
			// just merge the large segment into the next one.  Since
			// the segmented flag is set, the caller will be prepared to
			// open a longer-running db transaction.
			if err := batch.CopyInto(ret.toFlush); err != nil {
				return nil, err
			}
			c.accumulator = nil
			return ret, nil
		}

		batchCount := batch.Count()
		if c.count+batchCount >= c.Config.FlushSize {
			// If accumulating the data would create an over-large
			// batch, perform a flush.
			ret := &step{toFlush: c.accumulator}
			c.accumulator = &types.MultiBatch{}
			c.count = batchCount
			if err := batch.CopyInto(c.accumulator); err != nil {
				return nil, err
			}
			return ret, nil
		}

		// The data will fit, so just copy into the accumulator.
		if err := batch.CopyInto(c.accumulator); err != nil {
			return nil, err
		}
		c.count += batchCount

		// We can continue to accumulate data in subsequent passes.
		return &step{}, nil

	case <-timerC:
		// The flush timer has expired. If no timer was passed in,
		// this channel will be nil and this block can never execute.
		ret := &step{toFlush: c.accumulator}
		c.accumulator = &types.MultiBatch{}
		c.count = 0
		return ret, nil

	case <-ctx.Stopping():
		// Just exit if being shut down.
		return nil, nil
	}
}
