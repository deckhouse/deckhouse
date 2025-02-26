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
				w.WriteHeader(http.StatusOK)
			} else if r.Method == http.MethodPost && r.URL.Path == "/api/v1/build" {
				w.WriteHeader(http.StatusOK)
			} else if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/doc/TestModule" {
				w.WriteHeader(http.StatusOK)
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

	t.Run("loadDocArchive", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "Expected POST method")
			assert.Equal(t, "/api/v1/doc/TestModule/1.0.0", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		err := s.loadDocArchive(context.Background(), server.URL+"/api/v1/doc/TestModule/1.0.0", []byte("test"))
		assert.NoError(t, err, "loadDocArchive should not return an error")
	})

	t.Run("delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "Expected DELETE method")
			assert.Equal(t, "/api/v1/doc/TestModule", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		err := s.delete(context.Background(), server.URL[7:], "TestModule", []string{"alpha"})
		assert.NoError(t, err, "delete should not return an error")
	})

	t.Run("build", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "Expected POST method")
			assert.Equal(t, "/api/v1/build", r.URL.Path, "Unexpected URL path")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		err := s.build(context.Background(), server.URL+"/api/v1/build")
		assert.NoError(t, err, "build should not return an error")
	})
}
