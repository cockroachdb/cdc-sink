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

package cdc

// This file contains code repackaged from url.go.

import (
	"encoding/json"

	"github.com/cockroachdb/replicator/internal/types"
	"github.com/cockroachdb/replicator/internal/util/ident"
	"github.com/pkg/errors"
)

// parseNdjsonQueryMutation is a parseMutation function.
// We expect the CREATE CHANGE FEED INTO ... AS ... to use the following options:
// envelope="wrapped",format="json",diff
func (h *Handler) parseNdjsonQueryMutation(req *request, rawBytes []byte) (types.Mutation, error) {
	keys, err := h.getPrimaryKey(req)
	if err != nil {
		return types.Mutation{}, err
	}
	qp := queryPayload{
		keys: keys,
	}
	if err := json.Unmarshal(rawBytes, &qp); err != nil {
		return types.Mutation{}, err
	}
	return qp.AsMutation()
}

// getPrimaryKey returns a map that contains all the columns that make up the primary key
// for the target table and their ordinal position within the key.
func (h *Handler) getPrimaryKey(req *request) (*ident.Map[int], error) {
	if req.keys != nil {
		return req.keys, nil
	}
	table, ok := req.target.(ident.Table)
	if !ok {
		return nil, errors.Errorf("expecting ident.Table, got %T", req.target)
	}
	conveyor, err := h.Conveyors.Get(table.Schema())
	if err != nil {
		return nil, err
	}
	columns, ok := conveyor.Watcher().Get().Columns.Get(table)
	if !ok {
		return nil, errors.Errorf("table %q not found", table)
	}
	req.keys = &ident.Map[int]{}
	for i, col := range columns {
		if col.Primary {
			req.keys.Put(col.Name, i)
		}
	}
	return req.keys, nil
}
