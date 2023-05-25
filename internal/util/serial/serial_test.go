// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package serial

import (
	"testing"

	"github.com/cockroachdb/cdc-sink/internal/sinktest/all"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var errExpected = errors.New("expected")

type fakeRow struct{ err error }

var _ pgx.Row = (*fakeRow)(nil)

func (f *fakeRow) Scan(dest ...any) error { return f.err }

type fakeRows struct {
	err      error
	rowCount int
}

var _ pgx.Rows = (*fakeRows)(nil)

func (f *fakeRows) Close()                                       {}
func (f *fakeRows) Conn() *pgx.Conn                              { return nil }
func (f *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("") }
func (f *fakeRows) Err() error                                   { return f.err }
func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (f *fakeRows) RawValues() [][]byte                          { return nil }
func (f *fakeRows) Scan(...any) error                            { return f.err }
func (f *fakeRows) Values() ([]any, error)                       { return nil, f.err }

func (f *fakeRows) Next() bool {
	if f.rowCount == 0 {
		return false
	}
	f.rowCount--
	return true
}

type fakeUnlockable bool

func (u *fakeUnlockable) Unlock() {
	if *u {
		panic(errors.New("redundant unlock"))
	}
	*u = true
}
func (u *fakeUnlockable) Unlocked() bool { return bool(*u) }

func TestPool(t *testing.T) {
	a := assert.New(t)

	fixture, cancel, err := all.NewFixture()
	if !a.NoError(err) {
		return
	}
	defer cancel()

	ctx := fixture.Context
	p := &Pool{Pool: fixture.TargetPool.Pool}

	t.Run("begin commit rollback", func(t *testing.T) {
		a := assert.New(t)
		a.NoError(p.Begin(ctx))
		a.EqualError(p.Begin(ctx), txOpenMsg)

		a.NoError(p.Commit(ctx))
		a.EqualError(p.Commit(ctx), noTxMsg)

		a.NoError(p.Rollback(ctx))
	})

	t.Run("exec", func(t *testing.T) {
		a := assert.New(t)
		_, err := p.Exec(ctx, "select 1")
		a.EqualError(err, noTxMsg)

		a.NoError(p.Begin(ctx))
		_, err = p.Exec(ctx, "select 1")
		a.NoError(err)
		a.NoError(p.Rollback(ctx))
	})
	t.Run("query", func(t *testing.T) {
		a := assert.New(t)
		rows, err := p.Query(ctx, "select 1")
		a.EqualError(err, noTxMsg)
		a.Nil(rows)

		a.NoError(p.Begin(ctx))
		rows, err = p.Query(ctx, "select 1")
		a.NoError(err)

		// Other unlocking behavior tested in rows_unblocker_test.
		rows.Close()

		a.NoError(p.Rollback(ctx))
	})
	t.Run("queryRow", func(t *testing.T) {
		a := assert.New(t)
		row := p.QueryRow(ctx, "select 1")
		a.EqualError(row.Scan(), noTxMsg)

		a.NoError(p.Begin(ctx))
		row = p.QueryRow(ctx, "select 1")
		var x int
		a.NoError(row.Scan(&x))

		a.NoError(p.Rollback(ctx))
	})
}
