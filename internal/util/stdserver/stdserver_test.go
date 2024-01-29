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

package stdserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cockroachdb/cdc-sink/internal/util/auth/trust"
	"github.com/cockroachdb/cdc-sink/internal/util/diag"
	"github.com/cockroachdb/cdc-sink/internal/util/stopper"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestServer(t *testing.T) {
	r := require.New(t)

	cfg := &Config{
		BindAddr:           "127.0.0.1:9988",
		DisableAuth:        false,
		GenerateSelfSigned: true,
	}

	ctx := stopper.WithContext(context.Background())
	ctx.Go(func() error {
		select {
		case <-ctx.Stopping():
		case <-time.After(2 * time.Minute):
			ctx.Stop(0)
		}
		return nil
	})
	dg := diag.New(ctx)

	l, err := Listener(ctx, cfg, dg)
	r.NoError(err)

	tlsCfg, err := TLSConfig(cfg)
	r.NoError(err)

	// This handler will report short-reads.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(time.Duration(rand.Int63n((10 * time.Second).Nanoseconds())))
		_, err := io.ReadAll(request.Body)
		if err != nil {
			t.Logf("%v", err)
			writer.WriteHeader(400)
		} else {
			writer.WriteHeader(200)
		}
	})

	New(ctx, trust.New(), dg, l, mux, tlsCfg)

	client := &http.Client{
		Timeout: time.Second, // If non-zero, we see the stream cancels and short writes.
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 0}).DialContext,
			MaxConnsPerHost:     100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     time.Minute,
			ForceAttemptHTTP2:   true, // If false, the call to Do() (always?) returns an error when timeout hits.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for i := 0; i < 100; i++ {
		eg.Go(func() error {
			for !ctx.IsStopping() {
				var buf bytes.Buffer
				for i := 0; i < 1024*1024; i++ {
					fmt.Fprintf(&buf, "%d", i)
				}

				req, err := http.NewRequestWithContext(egCtx, "GET",
					fmt.Sprintf("https://%s/", l.Addr().String()), bytes.NewReader(buf.Bytes()))
				if err != nil {
					t.Logf("%v", err)
					continue
				}

			retry:
				for i := 0; i < 5; i++ {
					res, err := client.Do(req)
					if err != nil {
						// We often, but not always, see a
						// context-deadline message here when the server
						// fails to respond in a timely fasion.
						t.Logf("%v", err)
					} else if res.StatusCode != 200 {
						// This is the behavior that's problematic: a
						// short write to the server *sometimes* doesn't
						// return an error, but instead returns a
						// response. It's only a non-200 status due to
						// the handler code above.
						t.Logf("BAD STATUS %d", res.StatusCode)
					} else {
						//	t.Log("OK")
						break retry
					}
				}
			}
			return nil
		})
	}

	r.NoError(eg.Wait())
}
