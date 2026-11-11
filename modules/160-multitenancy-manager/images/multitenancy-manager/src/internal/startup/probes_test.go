/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package startup

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServeProbesWhile checks that the probes are answered while the wait runs, live but not ready,
// and that the address is free for the manager once the wait is over.
func TestServeProbesWhile(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	// No keep-alive: an idle connection of the client would outlive the test.
	probes := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	statuses := make(map[string]int)
	err = ServeProbesWhile(t.Context(), listener, func(ctx context.Context) error {
		for _, path := range []string{LivenessPath, ReadinessPath} {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+path, http.NoBody)
			require.NoError(t, err)
			resp, err := probes.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			statuses[path] = resp.StatusCode
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]int{
		LivenessPath:  http.StatusOK,
		ReadinessPath: http.StatusServiceUnavailable,
	}, statuses)

	again, err := net.Listen("tcp", address)
	require.NoError(t, err, "the manager binds the address of the probes after the wait")
	require.NoError(t, again.Close())
}

// TestServeProbesWhile_WaitFails checks that the error of the wait is returned and the probes stop.
func TestServeProbesWhile_WaitFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	waitErr := errors.New("the wait failed")

	err = ServeProbesWhile(t.Context(), listener, func(context.Context) error { return waitErr })

	require.ErrorIs(t, err, waitErr)
	again, err := net.Listen("tcp", address)
	require.NoError(t, err)
	require.NoError(t, again.Close())
}
