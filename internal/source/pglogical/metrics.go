// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pglogical

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dialFailureCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pglogical_dial_failure_total",
		Help: "the number of times we failed to create a replication connection",
	})
	dialSuccessCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pglogical_dial_success_total",
		Help: "the number of times we successfully dialed a replication connection",
	})
)
