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
	"github.com/cockroachdb/replicator/internal/script"
	"github.com/cockroachdb/replicator/internal/types"
	"github.com/google/wire"
)

// Set is used by Wire.
var Set = wire.NewSet(ProvideSequencer)

// ProvideSequencer is called by Wire.
func ProvideSequencer(
	loader *script.Loader, targetPool *types.TargetPool, watchers types.Watchers,
) *Sequencer {
	return &Sequencer{
		loader:     loader,
		targetPool: targetPool,
		watchers:   watchers,
	}
}
