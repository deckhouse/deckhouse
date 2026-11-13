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

package health

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/metrics"
)

// freeAddr returns a local address that no process listens on.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	return addr
}

// get returns the status code and the body of the path. It tries again
// until the server listens.
func get(t *testing.T, addr, path string) (int, string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for {
		resp, err := http.Get("http://" + addr + path)
		if err == nil {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			return resp.StatusCode, string(body)
		}

		if time.Now().After(deadline) {
			t.Fatalf("get %s: %v", path, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// One server serves the probes of kubelet and the metrics for kube-rbac-proxy.
func TestServerServesProbesAndMetrics(t *testing.T) {
	addr := freeAddr(t)
	reg := metrics.New("w1")

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- NewServer(addr, log.NewNop(), func() bool { return false }, func() bool { return true }, reg.Handler()).Run(ctx)
	}()

	want := `d8_fencing_agent_polls_total{node="w1"} 0`
	if code, body := get(t, addr, "/metrics"); code != http.StatusOK || !strings.Contains(body, want) {
		t.Errorf("/metrics = %d %q, want 200 with %s", code, body, want)
	}

	if code, _ := get(t, addr, "/healthz"); code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", code)
	}

	if code, _ := get(t, addr, "/readyz"); code != http.StatusServiceUnavailable {
		t.Errorf("/readyz = %d, want 503 when the agent is not ready", code)
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("Run returned %v after the end of ctx", err)
	}
}

// Another process holds the address. The agent cannot answer the probes, so
// Run stops with an error.
func TestServerFailsOnABusyAddress(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()

	alive := func() bool { return true }

	err = NewServer(busy.Addr().String(), log.NewNop(), alive, alive, http.NotFoundHandler()).Run(t.Context())
	if err == nil {
		t.Error("Run returned nil on a busy address, want an error")
	}
}
