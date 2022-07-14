// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cdc

// This file contains code repackaged from url.go.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"regexp"

	"github.com/cockroachdb/cdc-sink/internal/types"
	"github.com/cockroachdb/cdc-sink/internal/util/batches"
	"github.com/cockroachdb/cdc-sink/internal/util/hlc"
	"github.com/cockroachdb/cdc-sink/internal/util/ident"
	"github.com/pkg/errors"
)

// See https://www.cockroachlabs.com/docs/stable/create-changefeed.html#general-file-format
// Example: /targetDB/targetSchema/2020-04-02/202004022058072107140000000000000-56087568dba1e6b8-1-72-00000000-test_table-1.ndjson
// Format is: /[endpoint]/[date]/[timestamp]-[uniquer]-[topic]-[schema-id]
var (
	ndjsonRegex        = regexp.MustCompile(`^/(?P<targetDB>[^/]+)/(?P<targetSchema>[^/]+)/(?P<date>\d{4}-\d{2}-\d{2})/(?P<uniquer>.+)-(?P<topic>[^-]+)-(?P<schema_id>[^-]+).ndjson$`)
	ndjsonTargetDB     = ndjsonRegex.SubexpIndex("targetDB")
	ndjsonTargetSchema = ndjsonRegex.SubexpIndex("targetSchema")
	ndjsonTopic        = ndjsonRegex.SubexpIndex("topic")
)

func (h *Handler) parseNdjsonURL(url *url.URL, req *request) error {
	match := ndjsonRegex.FindStringSubmatch(url.Path)
	if match == nil {
		return errors.Errorf("can't parse url %s", url)
	}

	db := ident.New(match[ndjsonTargetDB])
	schema := ident.New(match[ndjsonTargetSchema])
	table, _, err := ident.Relative(db, schema, match[ndjsonTopic])
	if err != nil {
		return err
	}

	req.leaf = h.ndjson
	req.target = table
	return nil
}

// parseMutation takes a single line from an ndjson and extracts enough
// information to be able to persist it to the staging table.
func parseMutation(rawBytes []byte) (types.Mutation, error) {
	var payload struct {
		After   json.RawMessage `json:"after"`
		Key     json.RawMessage `json:"key"`
		Updated string          `json:"updated"`
	}

	// Large numbers are not turned into strings, so the UseNumber option for
	// the decoder is required.
	dec := json.NewDecoder(bytes.NewReader(rawBytes))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return types.Mutation{}, err
	}

	if payload.Updated == "" {
		return types.Mutation{},
			errors.New("CREATE CHANGEFEED must specify the 'WITH updated' option")
	}

	// Parse the timestamp into nanos and logical.
	ts, err := hlc.Parse(payload.Updated)
	if err != nil {
		return types.Mutation{}, err
	}

	return types.Mutation{
		Time: ts,
		Data: payload.After,
		Key:  payload.Key,
	}, nil
}

// ndjson parses an incoming block of ndjson files and stores the
// associated Mutations. This assumes that the underlying
// Stager will store duplicate values in an idempotent manner,
// should the request fail partway through.
func (h *Handler) ndjson(ctx context.Context, req *request) error {
	muts, release := batches.Mutation()
	defer release()
	target := req.target.(ident.Table)

	// In immediate mode, we want to apply the mutations immediately.
	// The CDC feed guarantees in-order delivery for individual rows.
	var flush func() error
	if h.Mode == IgnoreTX {
		applier, err := h.Appliers.Get(ctx, target)
		if err != nil {
			return err
		}
		flush = func() error {
			err := applier.Apply(ctx, h.Pool, muts)
			muts = muts[:0]
			return err
		}
	} else {
		store, err := h.Stores.Get(ctx, target)
		if err != nil {
			return err
		}
		flush = func() error {
			err := store.Store(ctx, h.Pool, muts)
			muts = muts[:0]
			return err
		}
	}

	scanner := bufio.NewScanner(req.body)
	for scanner.Scan() {
		buf := scanner.Bytes()
		if len(buf) == 0 {
			continue
		}
		mut, err := parseMutation(buf)
		if err != nil {
			return err
		}
		muts = append(muts, mut)
		if len(muts) == cap(muts) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
