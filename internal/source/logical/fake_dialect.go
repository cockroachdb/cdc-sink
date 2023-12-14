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

package logical

import (
	"github.com/cockroachdb/cdc-sink/internal/util/stamp"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/pkg/errors"
)

// See discussion in Factory.Immediate.
type fakeDialect struct{}

func (f *fakeDialect) ReadInto(_ *stopper.Context, _ chan<- Message, _ State) error {
	return errors.New("fake")
}

func (f *fakeDialect) Process(_ *stopper.Context, _ <-chan Message, _ Events) error {
	return errors.New("fake")
}

func (f *fakeDialect) ZeroStamp() stamp.Stamp {
	return &fakeStamp{}
}

type fakeStamp struct{}

func (f *fakeStamp) Less(_ stamp.Stamp) bool {
	return false
}
