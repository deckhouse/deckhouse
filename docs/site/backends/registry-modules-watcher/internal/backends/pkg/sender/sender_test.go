// Copyright 2025 Flant JSC
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

package sender

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"registry-modules-watcher/internal/backends"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestSender(t *testing.T) {
	logger := log.NewNop()
	s := New(logger)

	t.Run("Send", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/api/v1/doc/TestModule/1.0.0" {
				w.WriteHeader(http.StatusCreated)
			} else if r.Method == http.MethodPost && r.URL.Path == "/api/v1/build" {
				w.WriteHeader(http.StatusOK)
			} else if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/doc/TestModule" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		listBackends := map[string]struct{}{
			server.URL[7:]: {}, // remove "http://"
		}
		versions := []backends.DocumentationTask{
			{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				TarFile:         []byte("test"),
				Task:            backends.TaskCreate,
			},
		}

		err := s.Send(context.Background(), listBackends, versions)
		assert.NoError(t, err, "Send should not return an error")
	})

	t.Run("delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "Expected DELETE method")
			assert.Equal(t, "/api/v1/doc/TestModule", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		version := backends.DocumentationTask{
			Registry:        "TestReg",
			Module:          "TestModule",
			Version:         "1.0.0",
			ReleaseChannels: []string{"alpha"},
			TarFile:         []byte("test"),
			Task:            backends.TaskDelete,
		}

		err := s.delete(context.Background(), server.URL[7:], version)
		assert.NoError(t, err, "delete should not return an error")
	})

	t.Run("upload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "Expected POST method")
			assert.Equal(t, "/api/v1/doc/TestModule/1.0.0", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		version := backends.DocumentationTask{
			Registry:        "TestReg",
			Module:          "TestModule",
			Version:         "1.0.0",
			ReleaseChannels: []string{"alpha"},
			TarFile:         []byte("test"),
			Task:            backends.TaskCreate,
		}

		err := s.upload(context.Background(), server.URL[7:], version)
		assert.NoError(t, err, "upload should not return an error")
	})

	t.Run("build", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "Expected POST method")
			assert.Equal(t, "/api/v1/build", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		err := s.build(context.Background(), server.URL[7:])
		assert.NoError(t, err, "build should not return an error")
	})
}
