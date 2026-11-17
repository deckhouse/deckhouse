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

package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type cancellingLogWriter struct {
	buffer bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancellingLogWriter) Write(data []byte) (int, error) {
	var entry struct {
		Level string `json:"level"`
	}

	if err := json.Unmarshal(data, &entry); err != nil {
		return 0, err
	}

	n, err := w.buffer.Write(data)
	if entry.Level == "error" {
		w.cancel()
	}

	return n, err
}

func TestWatcherLogsEventErrors(t *testing.T) {
	tests := []struct {
		name    string
		object  any
		watch   func(*Watcher, context.Context)
		message string
	}{
		{
			name: "secret",
			object: &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
				Data:       map[string][]byte{".dockerconfigjson": []byte("test-value")},
			},
			watch:   (*Watcher).watchSecret,
			message: "Process secret event",
		},
		{
			name: "module source",
			object: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "deckhouse.io/v1alpha1",
				"kind":       "ModuleSource",
				"metadata":   map[string]any{"resourceVersion": "2"},
				"spec": map[string]any{"registry": map[string]any{
					"repo": "registry.test/modules", "dockerCfg": "dGVzdC12YWx1ZQ==",
				}},
			}},
			watch:   (*Watcher).watchModuleSources,
			message: "Process module source event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testWatcherLogsEventError(t, tt.object, tt.watch, tt.message)
		})
	}
}

func testWatcherLogsEventError(t *testing.T, object any, watch func(*Watcher, context.Context), message string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{"type": "ADDED", "object": object}); err != nil {
			t.Errorf("encode watch event: %v", err)
			return
		}

		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	output := &cancellingLogWriter{cancel: cancel}
	config := &rest.Config{Host: server.URL}
	client, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(config)
	require.NoError(t, err)

	watcher := &Watcher{
		k8sClient:        client,
		k8sDynamicClient: dynamicClient,
		logger:           log.NewLogger(log.WithOutput(output), log.WithHandlerType(log.JSONHandlerType)),
	}
	watch(watcher, ctx)

	decoder := json.NewDecoder(&output.buffer)

	var record map[string]any
	for decoder.More() {
		require.NoError(t, decoder.Decode(&record))

		if record["level"] == "error" {
			break
		}
	}

	require.Equal(t, "error", record["level"])
	require.Equal(t, message, record["msg"])
	require.Contains(t, record["error"], "invalid character")
	require.NotContains(t, record, "!BADKEY")
}
