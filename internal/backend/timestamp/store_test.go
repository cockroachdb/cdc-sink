// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package timestamp

import (
	"testing"

	"github.com/cockroachdb/cdc-sink/internal/backend/sinktest"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/stretchr/testify/assert"
)

func TestSwap(t *testing.T) {
	a := assert.New(t)
	ctx, dbInfo, cancel := sinktest.Context()
	a.NotEmpty(dbInfo.Version())
	defer cancel()

	targetDb, cancel, err := sinktest.CreateDb(ctx)
	if !a.NoError(err) {
		return
	}
	defer cancel()

	s, err := New(ctx, dbInfo.Pool(), DefaultTable)
	if !a.NoError(err) {
		return
	}

	const count = 10
	prev := hlc.Zero()
	for i := 0; i <= count; i++ {
		next := hlc.New(int64(1000*i), i)
		found, err := s.Swap(ctx, dbInfo.Pool(), targetDb.Raw(), next)
		if !a.NoError(err) {
			return
		}
		a.Equal(prev, found)
		prev = next
	}

	a.Equal(int64(1000*count), prev.Nanos())
	a.Equal(count, prev.Logical())
}
