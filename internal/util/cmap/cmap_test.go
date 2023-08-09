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

package cmap

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	r := require.New(t)

	mapper := func(s string) int {
		ret, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		return ret
	}

	m := New[string, int, rune](mapper)

	// Check empty state.
	r.Equal(0, m.Len())
	found, ok := m.Get("0")
	r.False(ok)
	r.Zero(found)

	m.Put("1", '1')
	m.Put("2", '2')
	m.Put("3", '3')
	r.Equal(3, m.Len())

	found, ok = m.Get("0")
	r.False(ok)
	r.Zero(found)

	// Basic lookups.
	found, ok = m.Get("1")
	r.True(ok)
	r.Equal('1', found)

	found, ok = m.Get("2")
	r.True(ok)
	r.Equal('2', found)

	found, ok = m.Get("2")
	r.True(ok)
	r.Equal('2', found)

	// Exact match.
	key, found, ok := m.Match("1")
	r.Equal("1", key)
	r.True(ok)
	r.Equal('1', found)

	// Canonicalized match.
	key, found, ok = m.Match("01")
	r.Equal("1", key)
	r.True(ok)
	r.Equal('1', found)

	// Canonical replacement.
	m.Put("001", 'R')
	r.Equal(3, m.Len())
	key, found, ok = m.Match("01")
	r.Equal("001", key)
	r.True(ok)
	r.Equal('R', found)

	count := 0
	r.NoError(m.Range(func(s string, r rune) error {
		count++
		return nil
	}))
	r.Equal(3, count)

	cpy := New[string, int, rune](mapper)
	m.CopyInto(cpy)
	r.Equal(m.Len(), cpy.Len())

	// Entries is not stable, since we don't require the C type to be
	// ordered.
	entries := m.Entries()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	r.Equal([]Entry[string, rune]{
		{"001", 'R'},
		{"2", '2'},
		{"3", '3'},
	}, entries)
}
