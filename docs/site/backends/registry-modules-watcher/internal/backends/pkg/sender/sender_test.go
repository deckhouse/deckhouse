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
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/log"

	"registry-modules-watcher/internal/backends"
)

func TestSender(t *testing.T) {
	logger := log.NewNop()
	s := New(logger, nil)

	MaxInterval = 10 * time.Millisecond

	t.Run("Send", func(t *testing.T) {
		t.Run("successful responses", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// nolint: gocritic
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

			t.Run("with create task", func(_ *testing.T) {
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

				s.Send(context.Background(), listBackends, versions)
			})

			t.Run("with delete task", func(_ *testing.T) {
				versions := []backends.DocumentationTask{
					{
						Registry:        "TestReg",
						Module:          "TestModule",
						Version:         "1.0.0",
						ReleaseChannels: []string{"alpha"},
						TarFile:         []byte("test"),
						Task:            backends.TaskDelete,
					},
				}

				s.Send(context.Background(), listBackends, versions)
			})
		})

		t.Run("error responses", func(t *testing.T) {
			t.Run("upload error with successful retry", func(t *testing.T) {
				// Counter to track number of retry attempts
				requestCount := 0

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && r.URL.Path == "/api/v1/doc/TestModule/1.0.0" {
						requestCount++
						if requestCount <= 2 {
							// Return error for first 2 attempts
							w.WriteHeader(http.StatusInternalServerError)
						} else {
							// Success on the 3rd attempt (after 2 retries)
							w.WriteHeader(http.StatusCreated)
						}
					} else {
						w.WriteHeader(http.StatusOK)
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

				s.Send(context.Background(), listBackends, versions)

				// Verify the sender attempted the expected number of requests
				assert.Equal(t, 3, requestCount,
					"Expected sender to make 3 requests total (1 initial + 2 retries) before success")
			})

			t.Run("build error with successful retry", func(t *testing.T) {
				// Counter to track number of retry attempts
				requestCount := 0

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost {
						switch r.URL.Path {
						case "/api/v1/doc/TestModule/1.0.0":
							w.WriteHeader(http.StatusCreated)
						case "/api/v1/build":
							requestCount++
							if requestCount <= 2 {
								// Return error for first 2 attempts
								w.WriteHeader(http.StatusInternalServerError)
							} else {
								// Success on the 3rd attempt (after 2 retries)
								w.WriteHeader(http.StatusOK)
							}
						}
					} else {
						w.WriteHeader(http.StatusOK)
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

				s.Send(context.Background(), listBackends, versions)

				// Verify the sender attempted the expected number of requests
				assert.Equal(t, 3, requestCount,
					"Expected sender to make 3 requests total (1 initial + 2 retries) before success")
			})

			t.Run("delete error with successful retry", func(t *testing.T) {
				// Counter to track number of retry attempts
				requestCount := 0

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/doc/TestModule" {
						requestCount++
						if requestCount <= 2 {
							// Return error for first 2 attempts
							w.WriteHeader(http.StatusInternalServerError)
						} else {
							// Success on the 3rd attempt (after 2 retries)
							w.WriteHeader(http.StatusNoContent)
						}
					} else {
						w.WriteHeader(http.StatusOK)
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
						Task:            backends.TaskDelete,
					},
				}

				s.Send(context.Background(), listBackends, versions)

				// Verify the sender attempted the expected number of requests
				assert.Equal(t, 3, requestCount,
					"Expected sender to make 3 requests total (1 initial + 2 retries) before success")
			})

			t.Run("connection error with successful retry", func(t *testing.T) {
				// Counter to track number of requests
				requestCount := 0

				// Create a server that initially refuses connections then works
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestCount++
					if requestCount <= 2 {
						// Close the connection without response for first 2 attempts
						hj, ok := w.(http.Hijacker)
						if !ok {
							t.Fatal("couldn't hijack connection")
						}
						conn, _, _ := hj.Hijack()
						conn.Close()
					} else {
						// Success on the 3rd attempt
						if r.URL.Path == "/api/v1/doc/TestModule/1.0.0" {
							w.WriteHeader(http.StatusCreated)
						} else {
							w.WriteHeader(http.StatusOK)
						}
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

				s.Send(context.Background(), listBackends, versions)

				// Verify the sender attempted the expected number of requests
				assert.Equal(t, 3, requestCount,
					"Expected sender to make 3 requests total (1 initial + 2 retries) before success")
			})
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("successful case", func(t *testing.T) {
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

		t.Run("http error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			version := backends.DocumentationTask{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				Task:            backends.TaskDelete,
			}

			err := s.delete(context.Background(), server.URL[7:], version)
			assert.Error(t, err, "delete should return an error on HTTP 500")
		})

		t.Run("connection error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Hijack and close connection to simulate network failure
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("couldn't hijack connection")
				}
				conn, _, _ := hj.Hijack()
				conn.Close()
			}))
			defer server.Close()

			version := backends.DocumentationTask{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				Task:            backends.TaskDelete,
			}

			err := s.delete(context.Background(), server.URL[7:], version)
			assert.Error(t, err, "delete should return an error on connection failure")
		})

		t.Run("invalid backend URL", func(t *testing.T) {
			version := backends.DocumentationTask{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				Task:            backends.TaskDelete,
			}

			err := s.delete(context.Background(), "invalid-host:8080", version)
			assert.Error(t, err, "delete should return an error with invalid backend URL")
		})
	})

	t.Run("upload", func(t *testing.T) {
		t.Run("successful case", func(t *testing.T) {
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

		t.Run("http error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
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
			assert.Error(t, err, "upload should return an error on HTTP 500")
		})

		t.Run("connection error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Hijack and close connection to simulate network failure
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("couldn't hijack connection")
				}
				conn, _, _ := hj.Hijack()
				conn.Close()
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
			assert.Error(t, err, "upload should return an error on connection failure")
		})

		t.Run("invalid backend URL", func(t *testing.T) {
			version := backends.DocumentationTask{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				TarFile:         []byte("test"),
				Task:            backends.TaskCreate,
			}

			err := s.upload(context.Background(), "invalid-host:8080", version)
			assert.Error(t, err, "upload should return an error with invalid backend URL")
		})
	})

	t.Run("build", func(t *testing.T) {
		t.Run("successful case", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method, "Expected POST method")
				assert.Equal(t, "/api/v1/build", r.URL.Path, "Unexpected URL path")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			err := s.build(context.Background(), server.URL[7:])
			assert.NoError(t, err, "build should not return an error")
		})

		t.Run("http error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			err := s.build(context.Background(), server.URL[7:])
			assert.Error(t, err, "build should return an error on HTTP 500")
		})

		t.Run("connection error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Hijack and close connection to simulate network failure
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("couldn't hijack connection")
				}
				conn, _, _ := hj.Hijack()
				conn.Close()
			}))
			defer server.Close()

			err := s.build(context.Background(), server.URL[7:])
			assert.Error(t, err, "build should return an error on connection failure")
		})

		t.Run("invalid backend URL", func(t *testing.T) {
			err := s.build(context.Background(), "invalid-host:8080")
			assert.Error(t, err, "build should return an error with invalid backend URL")
		})

		t.Run("retry behavior", func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				if requestCount <= 2 {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()

			err := s.build(context.Background(), server.URL[7:])
			assert.NoError(t, err, "build should succeed after retries")
			assert.Equal(t, 3, requestCount, "Expected 3 requests before success")
		})
	})
}
