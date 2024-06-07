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

package immediate

import (
	"github.com/cockroachdb/replicator/internal/sequencer"
	"github.com/cockroachdb/replicator/internal/sequencer/decorators"
	"github.com/cockroachdb/replicator/internal/types"
	"github.com/google/wire"
)

// Set is used by Wire.
var Set = wire.NewSet(
	ProvideImmediate,
)

// ProvideImmediate is called by Wire.
func ProvideImmediate(
	cfg *sequencer.Config,
	db *types.TargetPool,
	marker *decorators.Marker,
	once *decorators.Once,
	retryTarget *decorators.RetryTarget,
	stagers types.Stagers,
) *Immediate {
	return &Immediate{
		cfg:         cfg,
		marker:      marker,
		once:        once,
		retryTarget: retryTarget,
		stagers:     stagers,
		targetPool:  db,
	}
}
