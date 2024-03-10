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
	"fmt"
	"os"
	"testing"

	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/stretchr/testify/require"
)

const rewriteExpected = false

func TestRewriteIsFalse(t *testing.T) {
	require.False(t, rewriteExpected)
}

func TestTimeOrderTemplate(t *testing.T) {
	r := require.New(t)

	r.NotNil(unstage)

	sch := ident.MustSchema(ident.New("my_db"), ident.Public)
	tbl0 := ident.NewTable(sch, ident.New("tbl0"))
	tbl1 := ident.NewTable(sch, ident.New("tbl1"))
	tbl2 := ident.NewTable(sch, ident.New("tbl2"))
	tbl3 := ident.NewTable(sch, ident.New("tbl3"))

	staging := ident.MustSchema(ident.New("_cdc_sink"), ident.Public)

	tcs := []struct {
		name string
		data *templateData
	}{
		{
			name: "basic",
			data: &templateData{
				ScanLimit:     10_000,
				StagingSchema: staging,
				Targets:       []ident.Table{tbl0, tbl1, tbl2, tbl3},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			out, err := tc.data.Eval()
			r.NoError(err)

			filename := fmt.Sprintf("./testdata/%s.sql", tc.name)
			if rewriteExpected {
				r.NoError(os.WriteFile(filename, []byte(out), 0644))
			} else {
				data, err := os.ReadFile(filename)
				r.NoError(err)
				r.Equal(string(data), out)
			}
		})
	}
}
