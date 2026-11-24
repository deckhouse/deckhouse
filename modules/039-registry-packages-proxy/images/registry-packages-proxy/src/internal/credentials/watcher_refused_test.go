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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// forbiddenEvent is what a RetryWatcher hands over when the API server refuses the watch.
func forbiddenEvent(resource, name string) watch.Event {
	status := apierrors.NewForbidden(schema.GroupResource{Resource: resource}, name, nil).Status()

	return watch.Event{Type: watch.Error, Object: &status}
}

// An error event carries a Status, not the watched object, and is reported rather than crashing the
// proxy (job 9013941: `interface conversion: runtime.Object is *v1.Status, not *v1.Secret`).
func TestAnErrorEventIsReportedNotCast(t *testing.T) {
	watcher := NewWatcher(nil, nil, nil, 0, log.NewNop())

	err := watcher.processSecretEvent(forbiddenEvent("secrets", "deckhouse-registry"))
	require.Error(t, err)
	require.True(t, apierrors.IsForbidden(err), "the API server's reason must survive: %v", err)

	err = watcher.processModuleSourceEvent(forbiddenEvent("modulesources", ""))
	require.Error(t, err)
	require.True(t, apierrors.IsForbidden(err), "the API server's reason must survive: %v", err)
}

// On 403 a RetryWatcher stops for good, and the API server answers 403 to everyone for a few seconds
// after a restart. The watch must be started again, or a later change of the secret is never seen.
func TestARefusedWatchIsStartedAgain(t *testing.T) {
	period := rewatchPeriod
	rewatchPeriod = 10 * time.Millisecond
	t.Cleanup(func() { rewatchPeriod = period })

	var refused atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/api/v1/namespaces/d8-system/secrets" ||
			!strings.Contains(r.URL.Query().Get("fieldSelector"), "deckhouse-registry") {
			// Every other watch: open and silent.
			w.(http.Flusher).Flush()
			<-r.Context().Done()

			return
		}

		if refused.CompareAndSwap(false, true) {
			status := apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "deckhouse-registry", nil).Status()
			status.APIVersion, status.Kind = "v1", "Status"
			w.WriteHeader(http.StatusForbidden)
			if err := json.NewEncoder(w).Encode(status); err != nil {
				t.Errorf("encode status: %v", err)
			}

			return
		}

		secret := &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "deckhouse-registry", ResourceVersion: "2"},
			Data: map[string][]byte{
				"address": []byte("registry.test"),
				"path":    []byte("/deckhouse/ce"),
				"scheme":  []byte("https"),
			},
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"type": "ADDED", "object": secret}); err != nil {
			t.Errorf("encode watch event: %v", err)
			return
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	config := &rest.Config{Host: server.URL}
	client, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(config)
	require.NoError(t, err)

	watcher := NewWatcher(client, dynamicClient, nil, 0, log.NewLogger(log.WithOutput(io.Discard)))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		watcher.Watch(ctx)
	}()

	require.Eventually(t, func() bool {
		config, err := watcher.Get(registry.DefaultRepository)
		return err == nil && config.Repository == "registry.test/deckhouse/ce"
	}, 4*time.Second, 10*time.Millisecond, "the secret must be applied by the watch started after the refusal")
	require.True(t, refused.Load(), "the first watch must have been refused")

	cancel()
	<-done
}
