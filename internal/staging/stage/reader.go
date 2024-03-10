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

package stage

import (
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/notify"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/pkg/errors"
)

// offset provides fine-grained pagination within the records for a
// single table.
type offset struct {
	Key  json.RawMessage
	Time hlc.Time
}

func (o *offset) String() string {
	return fmt.Sprintf("%s @ %s", o.Key, o.Time)
}

// reader provides the implementation of [types.Stagers.Unstage].
type reader struct {
	accumulator *types.TemporalBatch        // Accumulates rows as we go.
	bounds      *notify.Var[hlc.Range]      // Timestamps to read within.
	count       int                         // The number of values in the accumulator.
	db          *types.StagingPool          // Access to the staging database.
	sqlQ        string                      // The SQL query that drives the reader.
	out         chan<- *types.StagingCursor // Communicate to the caller.
	segmentSize int                         // Upper bound on the size of data we'll send.
	targets     []ident.Table               // Names of target tables.

}

func newReader(
	bounds *notify.Var[hlc.Range],
	db *types.StagingPool,
	out chan<- *types.StagingCursor,
	sqlQ string,
	segmentSize int,
	targets []ident.Table,
) *reader {
	return &reader{
		bounds:      bounds,
		db:          db,
		sqlQ:        sqlQ,
		segmentSize: segmentSize,
		out:         out,
		targets:     targets,
	}
}

// run assumes it's being executed via its own goroutine.
func (r *reader) run(ctx *stopper.Context) {
	// These offsets will be populated after the first call to queryOnce
	// and mask the minimum bound. The minimum should, in general,
	// always lag behind where we're reading from.
	tableOffsets := &ident.TableMap[*offset]{}

	// We're not using stopvar.DoWhenChanged since we want to refresh
	// the bounds in the middle of the loop.
	for {
		bounds, changed := r.bounds.Get()
		// Skip nuisance idle message if there's nothing that could be
		// returned.
		if !bounds.Empty() {
			// Execute the query in a tight loop as long as we have
			// new data to emit.
			for hadRows := true; hadRows && !ctx.IsStopping(); {
				var err error
				tableOffsets, hadRows, err = r.queryOnce(ctx, bounds, tableOffsets)

				// If we couldn't read the data, send an error message
				// and return. Writing to the out channel may block if
				// there's a slow consumer.
				if err != nil {
					select {
					case r.out <- &types.StagingCursor{Error: err}:
					case <-ctx.Stopping():
					}
					return
				}

				// Refresh the bounds between reads, so we can extend
				// the window that we're reading. The tableOffsets map
				// will be populated at this point, so changes to the
				// minimum are ignored.
				bounds, changed = r.bounds.Get()
			}

			// Flush any still-buffered data.
			r.flush(ctx, false, hlc.Zero())

			// Send an idle message and wait for something interesting
			// to happen. Writing to the out channel may block if
			// there's a slow consumer.
			select {
			case r.out <- &types.StagingCursor{Idle: bounds}:
			case <-ctx.Stopping():
				return
			}
		}

		// Wait for something interesting to happen.
		select {
		case <-changed:
		case <-ctx.Stopping():
			return
		}
	}
}

// flush the current accumulator, if it exists, and prepare a new one.
// A zero time is a signal not to create a new accumulator.
func (r *reader) flush(ctx *stopper.Context, needsSegment bool, ts hlc.Time) {
	if r.accumulator != nil {
		cursor := &types.StagingCursor{
			Batch:     r.accumulator,
			Segmented: needsSegment,
		}
		select {
		case r.out <- cursor:
		case <-ctx.Stopping():
		}
	}
	if ts == hlc.Zero() {
		r.accumulator = nil
	} else {
		r.accumulator = &types.TemporalBatch{Time: ts}
		r.count = 0
	}
}

// queryOnce retrieves a limited number of rows. The returned offset map
// will allow the next call to queryOnce to pick up where it left off.
func (r *reader) queryOnce(
	ctx *stopper.Context, bounds hlc.Range, initialOffsets *ident.TableMap[*offset],
) (offsets *ident.TableMap[*offset], hadRows bool, _ error) {
	// We need a copy of the map in case we have a retryable error.
	offsets = &ident.TableMap[*offset]{}
	initialOffsets.CopyInto(offsets)

	nanoOffsets := make([]int64, len(r.targets))
	logicalOffsets := make([]int, len(r.targets))
	keyOffsets := make([]string, len(r.targets))

	for idx, tbl := range r.targets {
		offset, ok := offsets.Get(tbl)
		if ok {
			nanoOffsets[idx] = offset.Time.Nanos()
			logicalOffsets[idx] = offset.Time.Logical()
			keyOffsets[idx] = string(offset.Key)
		} else {
			nanoOffsets[idx] = bounds.Min().Nanos()
			logicalOffsets[idx] = bounds.Min().Logical()
		}
	}

	rows, err := r.db.Query(ctx,
		r.sqlQ,
		nanoOffsets,
		logicalOffsets,
		bounds.Max().Nanos(),
		bounds.Max().Logical(),
		keyOffsets)
	if err != nil {
		return nil, false, errors.Wrap(err, r.sqlQ)
	}
	defer rows.Close()

	for rows.Next() {
		hadRows = true

		var mut types.Mutation
		var tableIdx int
		var nanos int64
		var logical int
		if err := rows.Scan(&tableIdx, &nanos, &logical, &mut.Key, &mut.Data, &mut.Before); err != nil {
			return nil, false, errors.WithStack(err)
		}

		mut.Before, err = maybeGunzip(mut.Before)
		if err != nil {
			return nil, false, err
		}
		mut.Data, err = maybeGunzip(mut.Data)
		if err != nil {
			return nil, false, err
		}
		mut.Time = hlc.New(nanos, logical)

		table := r.targets[tableIdx]
		offsets.Put(table, &offset{
			Time: mut.Time,
			Key:  mut.Key,
		})

		// The flush method will also bootstrap a new reader.
		needsSegment := r.segmentSize > 0 && r.count >= r.segmentSize
		needsFlush := r.accumulator == nil || r.accumulator.Time != mut.Time || needsSegment
		if needsFlush {
			r.flush(ctx, needsSegment, mut.Time)
		}

		if err := r.accumulator.Accumulate(table, mut); err != nil {
			return nil, false, err
		}
		r.count++
	}
	return offsets, hadRows, rows.Err()
}
