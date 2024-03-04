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

import (
	"encoding/json"

	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/pkg/errors"
)

// labels
var (
	afterLabel  = ident.New("after")
	beforeLabel = ident.New("before")
	crdbLabel   = ident.New("__crdb__")
	updated     = ident.New("updated")
)

// queryPayload stores the payload sent by the client for
// a change feed that uses a query
type queryPayload struct {
	after     *ident.Map[json.RawMessage]
	before    *ident.Map[json.RawMessage]
	keys      *ident.Map[int]
	keyValues []json.RawMessage
	updated   hlc.Time
}

// AsMutation converts the QueryPayload into a types.Mutation
// Note: Phantom deletes are represented by a Mutation with empty keys.
func (q *queryPayload) AsMutation() (types.Mutation, error) {
	if q.after == nil && q.before == nil {
		return types.Mutation{}, nil
	}
	// The JSON marshaling errors should fall into the never-happens
	// category since we unmarshalled the values below.
	var after, before json.RawMessage
	if q.after != nil {
		var err error
		after, err = json.Marshal(q.after)
		if err != nil {
			return types.Mutation{}, errors.WithStack(err)
		}
	}
	if q.before != nil {
		var err error
		before, err = json.Marshal(q.before)
		if err != nil {
			return types.Mutation{}, errors.WithStack(err)
		}
	}
	key, err := json.Marshal(q.keyValues)
	if err != nil {
		return types.Mutation{}, errors.WithStack(err)
	}
	return types.Mutation{
		Before: before,
		Data:   after,
		Key:    key,
		Time:   q.updated,
	}, nil
}

// UnmarshalJSON reads a serialized JSON object,
// and extracts the updated timestamp and the values
// for all remaining fields.
// If QueryPayload is initialized with Keys that are expected in the data,
// UnmarshalJSON will extract and store them in keyValues slice.
// Supports json format with envelope='wrapped'.
// Examples:
// insert: {"after":{"k":2,"v":"a"},"before":null,"updated":"1.0"}
// update: {"after":{"k":2,"v":"a"},"before":{"k":2,"v":"b"},"updated":"1.0"}
// delete: {"after":null,"before":{"k":2,"v":"b"},"updated":"1.0"}
// phantom delete: {"after":null,"before":null,"updated":"1.0"}
func (q *queryPayload) UnmarshalJSON(data []byte) error {
	// Parse the payload into msg. We'll perform some additional
	// extraction on the data momentarily.
	var msg *ident.Map[json.RawMessage]
	if err := json.Unmarshal(data, &msg); err != nil {
		return errors.Wrap(err, "could not parse query payload")
	}
	// Check if it is a raw message. Raw messages have a `__crdb__` property.
	_, hasCrdb := msg.Get(crdbLabel)
	if hasCrdb {
		return errors.New(`raw enveloped is not supported. Use envelope="wrapped",format="json" in CREATE CHANGEFEED ... AS SELECT`)
	}
	ts, ok := msg.Get(updated)
	if !ok {
		return errors.Errorf("could not find timestamp in field %s while attempting to parse envelope=wrapped", updated)
	}
	if err := q.updated.UnmarshalJSON(ts); err != nil {
		return err
	}
	if after, ok := msg.Get(afterLabel); ok {
		if err := json.Unmarshal(after, &q.after); err != nil {
			return errors.Wrap(err, "could not parse 'after' payload")
		}
	}
	if before, ok := msg.Get(beforeLabel); ok {
		if err := json.Unmarshal(before, &q.before); err != nil {
			return errors.Wrap(err, "could not parse 'before' payload")
		}
	}
	// If q.after is nil, the it's a delete.
	if q.after == nil {
		msg = q.before
		// Changefeed might emit empty `after` and `before` properties
		// if it's a phantom delete. We just discard these messages.
		if msg == nil {
			return nil
		}
	} else {
		msg = q.after
	}
	// Extract PK values.
	q.keyValues = make([]json.RawMessage, q.keys.Len())
	return q.keys.Range(func(k ident.Ident, pos int) error {
		if msg == nil {
			return errors.New("missing primary keys")
		}
		v, ok := msg.Get(k)
		if !ok {
			return errors.Errorf("missing primary key: %s", k)
		}
		q.keyValues[pos] = v
		return nil
	})
}
