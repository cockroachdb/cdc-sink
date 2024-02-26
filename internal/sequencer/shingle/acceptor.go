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

package shingle

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cockroachdb/cdc-sink/internal/sequencer"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/cockroachdb/cdc-sink/internal/util/lockset"
	"github.com/cockroachdb/cdc-sink/internal/util/notify"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

type acceptor struct {
	*Shingle
	delegate types.MultiAcceptor
	order    lockset.Set[string]
}

var (
	_ sequencer.MarkingAcceptor = (*acceptor)(nil)
	_ types.MultiAcceptor       = (*acceptor)(nil)
)

// AcceptTableBatch implements [types.MultiAcceptor] and calls the
// delegate.
func (a *acceptor) AcceptTableBatch(
	ctx context.Context, batch *types.TableBatch, opts *types.AcceptOptions,
) error {
	return a.delegate.AcceptTableBatch(ctx, batch, opts)
}

// AcceptTemporalBatch implements [types.TemporalAcceptor] and calls the
// delegate.
func (a *acceptor) AcceptTemporalBatch(
	ctx context.Context, batch *types.TemporalBatch, opts *types.AcceptOptions,
) error {
	return a.delegate.AcceptTemporalBatch(ctx, batch, opts)
}

// AcceptMultiBatch executes each enclosed TemporalBatch in a concurrent
// fashion. Two batches may be executed concurrently if they have no
// overlapping primary keys.
func (a *acceptor) AcceptMultiBatch(
	ctx context.Context, batch *types.MultiBatch, opts *types.AcceptOptions,
) error {
	stopCtx := stopper.From(ctx)

	// Limit total concurrency to something reasonable.
	sem := semaphore.NewWeighted(int64(a.cfg.Parallelism))

	outcomes := make([]*notify.Var[*lockset.Status], len(batch.Data))
	for idx, sub := range batch.Data {
		idx, sub := idx, sub // Capture
		keys := batchKeys(sub)
		outcomes[idx], _ = a.order.Schedule(keys, func([]string) error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return errors.WithStack(err)
			}
			defer sem.Release(1)

			// We may have been delayed for some time, so we'll re-check
			// that it's OK to continue running.
			if stopCtx.IsStopping() {
				return context.Canceled
			}

			tx, err := a.target.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return errors.WithStack(err)
			}
			defer func() { _ = tx.Rollback() }()

			opts := opts.Copy()
			opts.TargetQuerier = tx
			if err := a.delegate.AcceptTemporalBatch(ctx, sub, opts); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				return errors.WithStack(err)
			}

			// Mark the mutations as having been applied.
			stagingTx, err := a.staging.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return errors.WithStack(err)
			}
			defer func() { _ = stagingTx.Rollback(context.Background()) }()

			if err := sub.Data.Range(func(table ident.Table, tableBatch *types.TableBatch) error {
				stager, err := a.stagers.Get(ctx, table)
				if err != nil {
					return err
				}
				return stager.MarkApplied(ctx, stagingTx, tableBatch.Data)
			}); err != nil {
				return err
			}
			return errors.WithStack(stagingTx.Commit(ctx))
		})
	}

	// Await completion of tasks.
outer:
	for _, outcome := range outcomes {
		for {
			status, changed := outcome.Get()
			if status.Success() {
				continue outer
			} else if status.Err() != nil {
				return status.Err()
			}
			select {
			case <-changed:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

// IsMarking implements [sequencer.MarkingAcceptor].
func (a *acceptor) IsMarking() bool { return true }

// Unwrap is an informal protocol to return the delegate.
func (a *acceptor) Unwrap() types.MultiAcceptor {
	return a.delegate
}

func batchKeys(batch *types.TemporalBatch) []string {
	var ret []string
	// Ignoring error because callback only returns nil.
	_ = batch.Data.Range(func(tbl ident.Table, tblData *types.TableBatch) error {
		for _, mut := range tblData.Data {
			ret = append(ret, fmt.Sprintf("%s:%s", tbl.Raw(), string(mut.Key)))
		}
		return nil
	})
	return ret
}
