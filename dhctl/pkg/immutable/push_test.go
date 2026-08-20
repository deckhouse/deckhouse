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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	require.Equal(t, []string{"/config"}, served, "a token demand is not a missing path: the legacy fallback must stay untried")
}

func TestPushNodeConfigNamesTheMachineWhenNeitherPathIsServed(t *testing.T) {
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

// One path, one request. The machine either serves /config or is not a machine
// waiting for a configuration, and probing a second path only hides that.
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

// Which of the two servers holds the port decides whether the machine may be
// handed a configuration at all, and the answer has to come from asking rather
// than from pushing a document to see what happens.
func TestWhoamiNamesWhoHoldsThePort(t *testing.T) {
	for _, want := range []string{WhoamiInstaller, WhoamiAgent} {
		var asked string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			asked = r.Method + " " + r.URL.Path
			_, _ = w.Write([]byte(want + "\n"))
		}))

		got, err := Whoami(t.Context(), strings.TrimPrefix(server.URL, "http://"))
		server.Close()

		require.NoError(t, err)
		require.Equal(t, want, got)
		require.Equal(t, "GET /whoami", asked, "the identity question must not change the machine")
	}
}

// An old image, or something else entirely, holds the port: that is not an
// identity, and must not read as one.
func TestWhoamiRefusesAServerThatDoesNotAnswerIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := Whoami(t.Context(), strings.TrimPrefix(server.URL, "http://"))

	require.ErrorIs(t, err, errPathUnknown)
}
