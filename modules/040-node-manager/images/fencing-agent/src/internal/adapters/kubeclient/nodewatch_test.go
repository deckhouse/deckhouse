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

package kubeclient

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type storeRecorder struct {
	mu    sync.Mutex
	peers map[string]domain.Peer
}

func newStoreRecorder() *storeRecorder {
	return &storeRecorder{peers: make(map[string]domain.Peer)}
}

func (s *storeRecorder) Upsert(peer domain.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peer.Name] = peer
}

func (s *storeRecorder) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, name)
}

func (s *storeRecorder) snapshot() map[string]domain.Peer {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]domain.Peer, len(s.peers))
	for k, v := range s.peers {
		out[k] = v
	}

	return out
}

func (s *storeRecorder) eventually(t *testing.T, check func(map[string]domain.Peer) bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if check(s.snapshot()) {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("%s; final state: %v", msg, s.snapshot())
}

func TestNodeWatcherFeedsStore(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		node("worker-1", "worker", internal("10.0.0.1")),
		node("master-1", "master", internal("10.0.0.9")),
	)...)

	store := newStoreRecorder()

	watcher, err := NewNodeWatcher(client, "worker", store, log.NewNop())
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		watcher.Run(ctx)
	}()

	if !watcher.WaitForSync(ctx) {
		t.Fatal("cache did not sync")
	}

	// Initial fill: the label selector keeps foreign groups out (the fake
	// applies selectors on LIST, so this is the only phase that can assert it).
	store.eventually(t, func(peers map[string]domain.Peer) bool {
		_, hasWorker := peers["worker-1"]
		_, hasMaster := peers["master-1"]

		return hasWorker && !hasMaster && len(peers) == 1
	}, "initial fill must contain exactly the own group")

	// A new Node of the group appears: quorum input must grow without restart.
	if _, err := client.CoreV1().Nodes().Create(t.Context(), node("worker-2", "worker", internal("10.0.0.2")), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	store.eventually(t, func(peers map[string]domain.Peer) bool {
		return len(peers) == 2
	}, "added node must reach the store")

	// The InternalIP changes: the stored peer must follow.
	updated := node("worker-2", "worker", internal("10.0.9.9"))
	if _, err := client.CoreV1().Nodes().Update(t.Context(), updated, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	store.eventually(t, func(peers map[string]domain.Peer) bool {
		return peers["worker-2"].IP == "10.0.9.9"
	}, "updated address must reach the store")

	// The Node is deleted: it must leave the expected membership.
	if err := client.CoreV1().Nodes().Delete(t.Context(), "worker-2", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete node: %v", err)
	}

	store.eventually(t, func(peers map[string]domain.Peer) bool {
		_, exists := peers["worker-2"]

		return !exists
	}, "deleted node must leave the store")

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop on context cancel")
	}
}
