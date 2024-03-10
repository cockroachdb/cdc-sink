// Copyright 2023 The Cockroach Authors
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
	"context"
	"sync"

	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
)

type factory struct {
	db        *types.StagingPool
	stagingDB ident.Schema
	stop      *stopper.Context

	mu struct {
		sync.RWMutex
		instances *ident.TableMap[*stage]
	}
}

var _ types.Stagers = (*factory)(nil)

// Get returns a memoized instance of a stage for the given table.
func (f *factory) Get(_ context.Context, target ident.Table) (types.Stager, error) {
	if ret := f.getUnlocked(target); ret != nil {
		return ret, nil
	}
	return f.createUnlocked(target)
}

func (f *factory) createUnlocked(table ident.Table) (*stage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if ret := f.mu.instances.GetZero(table); ret != nil {
		return ret, nil
	}

	ret, err := newStage(f.stop, f.db, f.stagingDB, table)
	if err == nil {
		f.mu.instances.Put(table, ret)
	}
	return ret, err
}

func (f *factory) getUnlocked(table ident.Table) *stage {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.mu.instances.GetZero(table)
}

// Read implements types.Stagers.
func (f *factory) Read(
	ctx *stopper.Context, q *types.StagingQuery,
) (<-chan *types.StagingCursor, error) {
	// Ensure all staging tables do, in fact, exist.
	for _, table := range q.Targets {
		if _, err := f.Get(ctx, table); err != nil {
			return nil, err
		}
	}

	sqlQ, err := (&templateData{
		ScanLimit:     q.SweepLimit,
		StagingSchema: f.stagingDB,
		Targets:       q.Targets,
	}).Eval()
	if err != nil {
		return nil, err
	}

	// In the worst case, there's one row per distinct timestamp.
	out := make(chan *types.StagingCursor, q.SweepLimit)

	r := newReader(q.Bounds, f.db, out, sqlQ, q.SegmentSize, q.Targets)
	ctx.Go(func() error {
		defer close(out)
		r.run(ctx)
		return nil
	})
	return out, nil
}
