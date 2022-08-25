// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// Package script contains support for loading configuration scripts
// built as JavaScript programs.
package script

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/target/apply"
	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

// A Dispatch function receives a source mutation and assigns mutations
// to some number of downstream tables. Dispatch functions are
// internally synchronized to ensure single-threaded access to the
// underlying JS VM.
type Dispatch func(ctx context.Context, mutation types.Mutation) (map[ident.Table][]types.Mutation, error)

// dispatchTo returns a Dispatch which assigns all mutations to the
// given target.
func dispatchTo(target ident.Table) Dispatch {
	return func(_ context.Context, mut types.Mutation) (map[ident.Table][]types.Mutation, error) {
		return map[ident.Table][]types.Mutation{
			target: {mut},
		}, nil
	}
}

// A Map function may modify the mutations that are applied to a
// specific table. The boolean value will be false if the input mutation
// should be discarded. Map functions are internally synchronized to
// ensure single-threaded access to the underlying JS VM.
type Map func(ctx context.Context, mut types.Mutation) (types.Mutation, bool, error)

var identity Map = func(_ context.Context, mut types.Mutation) (types.Mutation, bool, error) {
	return mut, true, nil
}

// A Source holds user-provided configuration options for a
// generic data-source.
type Source struct {
	// The table to apply incoming deletes to; this assumes that the
	// target schema is using FK's with ON DELETE CASCADE.
	DeletesTo ident.Table
	Mapper    Dispatch
}

// A Target holds user-provided configuration options
// for a target table.
type Target struct {
	apply.Config
	Map Map
}

// UserScript encapsulates a user-provided configuration expressed as a
// JavaScript program.
type UserScript struct {
	Sources map[ident.Ident]*Source
	Targets map[ident.Table]*Target

	rt      *goja.Runtime // The JavaScript VM. See execJS.
	rtMu    sync.Mutex    // Serialize access to the VM.
	target  ident.Schema  // The schema being populated.
	watcher types.Watcher // Access to target schema.
}

// bind validates the user configuration against the target schema and
// creates the public facade around JS callbacks.
func (s *UserScript) bind(loader *Loader) error {
	for sourceName, bag := range loader.sources {
		src := &Source{}
		s.Sources[ident.New(sourceName)] = src

		switch {
		case bag.Dispatch != nil:
			if bag.DeletesTo != "" {
				src.DeletesTo = ident.NewTable(
					s.target.Database(), s.target.Schema(), ident.New(bag.DeletesTo))
			}
			src.Mapper = s.bindDispatch(sourceName, bag.Dispatch)

		case bag.Target != "":
			dest := ident.NewTable(s.target.Database(), s.target.Schema(), ident.New(bag.Target))
			src.DeletesTo = dest
			src.Mapper = dispatchTo(dest)

		default:
			return errors.Errorf("configureSource(%q): dispatch or target required", sourceName)
		}
	}

	for tableName, bag := range loader.targets {
		table := ident.NewTable(s.target.Database(), s.target.Schema(), tableName)
		tgt := &Target{Config: *apply.NewConfig()}
		s.Targets[table] = tgt

		for _, cas := range bag.CASColumns {
			tgt.CASColumns = append(tgt.CASColumns, ident.New(cas))
		}
		for k, v := range bag.Deadlines {
			d, err := time.ParseDuration(v)
			if err != nil {
				return errors.Wrapf(err, "configureTable(%q)", tableName)
			}
			tgt.Deadlines[ident.New(k)] = d
		}
		for k, v := range bag.Exprs {
			tgt.Exprs[ident.New(k)] = v
		}
		if bag.Extras != "" {
			tgt.Extras = ident.New(bag.Extras)
		}
		if bag.Map == nil {
			tgt.Map = identity
		} else {
			tgt.Map = s.bindMap(table, bag.Map)
		}
		for k, v := range bag.Ignore {
			if v {
				tgt.Ignore[ident.New(k)] = true
			}
		}
	}

	return nil
}

// bindDispatch exports a user-provided function as a Dispatch.
func (s *UserScript) bindDispatch(fnName string, dispatch dispatchJS) Dispatch {
	return func(ctx context.Context, mut types.Mutation) (map[ident.Table][]types.Mutation, error) {
		data := make(map[string]interface{})
		if err := json.Unmarshal(mut.Data, &data); err != nil {
			return nil, errors.WithStack(err)
		}

		var dispatches map[string][]map[string]interface{}
		if err := s.execJS(ctx, func() (err error) {
			dispatches, err = dispatch(data)
			return err
		}); err != nil {
			return nil, err
		}

		// Filtered out, return an empty map.
		if len(dispatches) == 0 {
			return map[ident.Table][]types.Mutation{}, nil
		}

		// Serialize mutations back to JSON.
		ret := make(map[ident.Table][]types.Mutation, len(dispatches))
		for tblName, jsDocs := range dispatches {
			tbl := ident.NewTable(s.target.Database(), s.target.Schema(), ident.New(tblName))
			tblMuts := make([]types.Mutation, len(jsDocs))
			ret[tbl] = tblMuts
			for idx, jsDoc := range jsDocs {
				colData, ok := s.watcher.Snapshot(s.target)[tbl]
				if !ok {
					return nil, errors.Errorf(
						"dispatch function %s returned unknown table %s", fnName, tbl)
				}

				// Extract the revised PK
				var jsKey []interface{}
				for _, col := range colData {
					if col.Primary {
						keyVal, ok := jsDoc[col.Name.Raw()]
						if !ok {
							return nil, errors.Errorf(
								"dispatch funcion %s omitted value for PK %s", fnName, col.Name)
						}
						jsKey = append(jsKey, keyVal)
					}
				}

				dataBytes, err := json.Marshal(jsDoc)
				if err != nil {
					return nil, err
				}

				keyBytes, err := json.Marshal(jsKey)
				if err != nil {
					return nil, err
				}

				tblMuts[idx] = types.Mutation{
					Data: dataBytes,
					Key:  keyBytes,
					Time: mut.Time,
				}
			}
		}

		return ret, nil
	}
}

// bindMap exports a user-provided function as a Map.
func (s *UserScript) bindMap(table ident.Table, mapper mapJS) Map {
	return func(ctx context.Context, mut types.Mutation) (types.Mutation, bool, error) {
		data := make(map[string]interface{})
		if err := json.Unmarshal(mut.Data, &data); err != nil {
			return mut, false, errors.WithStack(err)
		}

		var mapped map[string]interface{}
		if err := s.execJS(ctx, func() (err error) {
			mapped, err = mapper(data)
			return err
		}); err != nil {
			return mut, false, err
		}

		// Filtered out.
		if len(mapped) == 0 {
			return mut, false, nil
		}

		dataBytes, err := json.Marshal(mapped)
		if err != nil {
			return mut, false, errors.WithStack(err)
		}

		colData, ok := s.watcher.Snapshot(s.target)[table]
		if !ok {
			return mut, false, errors.Errorf("map missing schema data for %s", table)
		}

		var jsKey []interface{}
		for _, colData := range colData {
			if colData.Primary {
				keyVal, ok := mapped[colData.Name.Raw()]
				if !ok {
					return mut, false, errors.Errorf(
						"map document missing value for PK column %s", colData.Name)
				}
				jsKey = append(jsKey, keyVal)
			}
		}

		keyBytes, err := json.Marshal(jsKey)
		if err != nil {
			return mut, false, errors.WithStack(err)
		}

		return types.Mutation{Data: dataBytes, Key: keyBytes, Time: mut.Time}, true, nil
	}
}

// execJS ensures that the callback has exclusive access to the JS VM.
// The JS execution will be interrupted when the context is canceled.
func (s *UserScript) execJS(ctx context.Context, fn func() error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.rtMu.Lock()
	s.rt.ClearInterrupt()
	go func() {
		defer s.rtMu.Unlock()
		<-ctx.Done()
		s.rt.Interrupt(ctx.Err())
	}()
	return fn()
}
