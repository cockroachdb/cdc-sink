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

package script

import (
	"context"

	"github.com/cockroachdb/cdc-sink/internal/script"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
)

// acceptor implements [types.TableAcceptor] and intercepts mutations
// for dispatch, mapping, or user-defined apply functions.
type acceptor struct {
	delegate   types.TableAcceptor
	group      *types.TableGroup
	justMap    bool
	userScript *script.UserScript
	watchers   types.Watchers
}

var _ types.TableAcceptor = (*acceptor)(nil)

// AcceptTableBatch implements [types.TableAcceptor]. It will invoke
// user-defined dispatch and/or map functions.
func (a *acceptor) AcceptTableBatch(
	ctx context.Context, batch *types.TableBatch, opts *types.AcceptOptions,
) error {
	// We're looping around from the bottom of this method.
	if a.justMap {
		return a.doMap(ctx, batch, opts)
	}

	// No configuration, just send down the line.
	source, ok := a.userScript.Sources.Get(a.group.Name)
	if !ok {
		return a.doMap(ctx, batch, opts)
	}

	return a.doDispatch(ctx, source, batch, opts)
}

// Unwrap is an informal protocol to return the delegate.
func (a *acceptor) Unwrap() types.TableAcceptor {
	return a.delegate
}

func (a *acceptor) doDispatch(
	ctx context.Context, source *script.Source, batch *types.TableBatch, opts *types.AcceptOptions,
) error {
	// We're going to construct a new batch from the dispatched mutations.
	nextBatch := &types.MultiBatch{}

	for _, mutToDispatch := range batch.Data {
		// Separate deletes from upserts for routing.
		if mutToDispatch.IsDelete() {
			deletesTo := source.DeletesTo
			if deletesTo.Empty() {
				deletesTo = batch.Table
			}
			if err := nextBatch.Accumulate(deletesTo, mutToDispatch); err != nil {
				return err
			}
			continue
		}

		script.AddMeta(a.group.Name.Raw(), batch.Table, &mutToDispatch)
		// Call the user function to see what mutations(s) go into which table(s).
		dispatched, err := source.Dispatch(ctx, mutToDispatch)
		if err != nil {
			return err
		}
		// Push the mutations into the replacement batch.
		if err := dispatched.Range(func(table ident.Table, muts []types.Mutation) error {
			for _, dispatchedMut := range muts {
				// If the time were unset, it would trigger an error.
				dispatchedMut.Time = mutToDispatch.Time
				if err := nextBatch.Accumulate(table, dispatchedMut); err != nil {
					return err
				}
			}
			return err
		}); err != nil {
			return err
		}
	}
	// Calls to source.Dispatch may remove mutations.
	// If the replacement batch is empty, we are done.
	if nextBatch.Count() == 0 {
		return nil
	}
	// Drop the source so that we'll always call doMap.
	cpy := *a
	cpy.justMap = true

	return types.OrderedAcceptorFrom(&cpy, a.watchers).AcceptMultiBatch(ctx, nextBatch, opts)
}

func (a *acceptor) doMap(
	ctx context.Context, batch *types.TableBatch, opts *types.AcceptOptions,
) error {
	target, ok := a.userScript.Targets.Get(batch.Table)
	if ok && target.Map != nil {
		mapped := batch.Empty()
		mapped.Data = make([]types.Mutation, 0, len(batch.Data))
		for _, mut := range batch.Data {
			if mut.IsDelete() {
				if target.DeleteKey == nil {
					mapped.Data = append(mapped.Data, mut)
				} else {
					next, keep, err := target.DeleteKey(ctx, mut)
					if err != nil {
						return err
					}
					if keep {
						mapped.Data = append(mapped.Data, next)
					}
				}
				continue
			}
			script.AddMeta(a.group.Name.Raw(), batch.Table, &mut)
			next, keep, err := target.Map(ctx, mut)
			if err != nil {
				return err
			}
			if !keep {
				continue
			}
			mapped.Data = append(mapped.Data, next)
		}
		batch = mapped
	}

	// Delegate to user-provided logic.
	if target.Acceptor != nil {
		return target.Acceptor.AcceptTableBatch(ctx, batch, opts)
	}

	// Otherwise, continue down the standard path.
	return a.delegate.AcceptTableBatch(ctx, batch, opts)
}