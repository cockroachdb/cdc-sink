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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestSafeValue(t *testing.T) {
	rt := goja.New()
	now := time.UnixMilli(1708731562135).UTC()

	tcs := []struct {
		input      any
		exportType string
		err        string
		jsString   string
		test       func(a *assert.Assertions, value goja.Value)
	}{
		{
			input:      maxInt + 1,
			exportType: "string",
			jsString:   fmt.Sprintf("%d", maxInt+1),
		},
		{
			input:      json.Number("12345"),
			exportType: "string",
			jsString:   "12345",
		},
		{
			input:      now,
			exportType: "string",
			jsString:   "2024-02-23T23:39:22.135Z",
			test: func(a *assert.Assertions, _ goja.Value) {
				a.Equal("2024-02-23 23:39:22.135 +0000 UTC", now.String())
			},
		},
		{
			input:      []any{now, now},
			exportType: "[]interface {}",
			jsString:   "2024-02-23T23:39:22.135Z,2024-02-23T23:39:22.135Z",
		},
		{
			input:      3.141592,
			exportType: "string",
			jsString:   "3.141592",
		},
		{
			input:      json.RawMessage("    3.141592"),
			exportType: "string",
			jsString:   "3.141592",
		},
		{
			input:      json.RawMessage(`  "    3.141592"`),
			exportType: "string",
			jsString:   "    3.141592",
		},
		{
			input:      json.RawMessage("true"),
			exportType: "bool",
			jsString:   "true",
		},
		{
			input:      json.RawMessage("false"),
			exportType: "bool",
			jsString:   "false",
		},
		{
			input:      json.RawMessage(`{"BigInt":9007199254740995}`),
			exportType: "map[string]interface {}",
			jsString:   "[object Object]",
			test: func(a *assert.Assertions, value goja.Value) {
				obj := value.(*goja.Object)
				a.Equal(`9007199254740995`, obj.Get("BigInt").String())
			},
		},
		{
			input:      json.RawMessage(`[9007199254740995]`),
			exportType: "[]interface {}",
			jsString:   "9007199254740995",
			test: func(a *assert.Assertions, value goja.Value) {
				obj := value.(*goja.Object)
				a.Equal(`9007199254740995`, obj.Get("0").String())
			},
		},
		{
			input: json.RawMessage(`     `),
			err:   "unexpected end of JSON input",
		},
	}

	for idx, tc := range tcs {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			a := assert.New(t)
			value, err := safeValue(rt, tc.input)
			if tc.err != "" {
				a.ErrorContains(err, tc.err)
			} else if a.NoError(err) {
				a.Equal(tc.exportType, value.ExportType().String())
				a.Equal(tc.jsString, value.String())
				if tc.test != nil {
					tc.test(a, value)
				}
			}
		})
	}
}
