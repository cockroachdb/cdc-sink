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

// Package kafka contains a command to perform logical replication
// from a kafka feed.
package kafka

import (
	"github.com/cockroachdb/field-eng-powertools/stopper"
	"github.com/cockroachdb/replicator/internal/source/kafka"
	"github.com/cockroachdb/replicator/internal/util/stdlogical"
	"github.com/spf13/cobra"
)

// Command returns the pglogical subcommand.
func Command() *cobra.Command {
	cfg := &kafka.Config{}
	return stdlogical.New(&stdlogical.Template{
		Config: cfg,
		Short:  "start a kafka replication feed",
		Start: func(ctx *stopper.Context, cmd *cobra.Command) (any, error) {
			return kafka.Start(ctx, cfg)
		},
		Use: "kafka",
	})
}
