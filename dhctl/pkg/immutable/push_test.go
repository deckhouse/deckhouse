// Copyright 2026 Flant JSC
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

package immutable

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPushNodeConfigDeliversTheDocument(t *testing.T) {
	var (
		method, path string
		body         []byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))

	require.NoError(t, err)
	require.Equal(t, http.MethodPut, method)
	require.Equal(t, "/config", path)
	require.Equal(t, "kind: NodeConfig\n", string(body))
}

func TestPushNodeConfigNamesAnInstalledNode(t *testing.T) {
	var served []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))

	require.ErrorContains(t, err, "already installed")
	require.ErrorIs(t, err, ErrMaintenanceTokenRequired, "the caller tells this terminal refusal from a transient one by the sentinel")
	require.Equal(t, []string{"/config"}, served, "a token demand is terminal: no second request follows it")
}

func TestPushNodeConfigNamesTheMachineWhenThePathIsNotServed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")

	err := PushNodeConfig(t.Context(), address, []byte("kind: NodeConfig\n"))

	require.ErrorIs(t, err, errPathUnknown)
	require.ErrorContains(t, err, address)
	require.ErrorContains(t, err, "/config")
}

// One path, one request: the machine either serves /config or is not waiting
// for a configuration at all.
func TestPushNodeConfigAsksOnePath(t *testing.T) {
	var served []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))

	require.NoError(t, err)
	require.Equal(t, []string{"/config"}, served)
}

func TestPushNodeConfigQuotesTheRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("config partition is read-only"))
	}))
	defer server.Close()

	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))

	require.ErrorContains(t, err, "500")
	require.ErrorContains(t, err, "config partition is read-only")
}

// A machine waiting for its configuration sits on the provisioning network, and
// what it is handed carries the bootstrap token, the cluster CA and the master's
// TLS serving key. A client without a transport of its own is the process-wide
// http.DefaultTransport, which sends all of that to whatever HTTP_PROXY names —
// and dhctl's own environment carries one often enough to forward it to
// terraform (pkg/infrastructure/terraform/cmd.go).
func TestPushNodeConfigKeepsOffTheProcessWideTransport(t *testing.T) {
	var reached bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	shared := &refusingTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = shared
	t.Cleanup(func() { http.DefaultTransport = original })

	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))

	require.NoError(t, err)
	require.Zero(t, shared.requests,
		"the push went through http.DefaultTransport, which proxies through HTTP_PROXY and pools its connections process-wide")
	require.True(t, reached, "the machine was handed nothing")
}

// refusingTransport stands in for http.DefaultTransport and carries nothing.
type refusingTransport struct{ requests int }

func (r *refusingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.requests++
	return nil, errors.New("http.DefaultTransport must carry no payload")
}

// do hands the response back for the caller to read, so its
// CloseIdleConnections runs before the body is: what makes that safe is that the
// call also marks the transport "close newly idle connections", so the socket is
// closed when the caller closes the body. Drop the call and it is leaked instead.
func TestPushNodeConfigLeavesNoOpenConnection(t *testing.T) {
	var (
		mu     sync.Mutex
		states []http.ConnState
	)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("config partition is read-only"))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		mu.Lock()
		defer mu.Unlock()
		states = append(states, state)
	}
	server.Start()
	defer server.Close()

	// The refusal path is the one that reads the body to the end, which is what
	// leaves a socket the transport would otherwise park in its idle pool.
	err := PushNodeConfig(t.Context(), strings.TrimPrefix(server.URL, "http://"), []byte("kind: NodeConfig\n"))
	require.ErrorContains(t, err, "500")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(states, http.StateClosed)
	}, 5*time.Second, 10*time.Millisecond,
		"the push left its connection open: the socket sits in the idle pool of a transport nothing drains")
}
