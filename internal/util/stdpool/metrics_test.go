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

package stdpool_test

import (
	"testing"

	"github.com/cockroachdb/replicator/internal/sinktest/base"
	"github.com/stretchr/testify/require"
)

// This test verifies that PublishMetrics can be called with the same
// destination host multiple times without panicking.
func TestConcurrentRegistration(t *testing.T) {
	r := require.New(t)

	_, err := base.NewFixture(t)
	r.NoError(err)

	_, err = base.NewFixture(t)
	r.NoError(err)
}
