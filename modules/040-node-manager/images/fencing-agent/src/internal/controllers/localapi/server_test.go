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

package localapi

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

func start(t *testing.T) (*Server, *http.Client, string) {
	t.Helper()

	// A unix socket path is limited to about 100 bytes, t.TempDir is longer.
	dir, err := os.MkdirTemp("", "fa")
	if err != nil {
		t.Fatalf("make temp dir: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "a.sock")
	server := New(path, "w1", log.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- server.Run(ctx) }()

	t.Cleanup(func() {
		cancel()

		if err := <-done; err != nil {
			t.Errorf("run: %v", err)
		}
	})

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		},
	}}

	waitAccept(path)

	return server, client, path
}

// waitAccept waits until the socket accepts connections. The socket file
// appears before the listen starts, so a dial right after the file appears
// can get "connection refused".
func waitAccept(path string) {
	for range 200 {
		if conn, err := net.Dial("unix", path); err == nil {
			_ = conn.Close()

			return
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func read(t *testing.T, client *http.Client) v1alpha1.FencingAgentNodeView {
	t.Helper()

	resp, err := client.Get("http://agent/")
	if err != nil {
		t.Fatalf("get view: %v", err)
	}
	defer resp.Body.Close()

	var view v1alpha1.FencingAgentNodeView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatalf("decode view: %v", err)
	}

	return view
}

func TestViewHasTheKubernetesAPISection(t *testing.T) {
	server, client, path := start(t)

	empty := read(t, client)
	if empty.Kind != "FencingAgentNodeView" || empty.Spec.NodeName != "w1" {
		t.Errorf("view is %+v", empty)
	}

	// The server sets the mode before it serves, so the first answer shows that
	// the mode is set.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket mode is %o, want 600", perm)
	}

	if empty.Status.KubernetesAPI.LastReadAt != nil || empty.Status.KubernetesAPI.LastVisibleAt != nil {
		t.Errorf("view has times before the first read: %+v", empty.Status.KubernetesAPI)
	}

	visibleAt := time.Date(2026, time.June, 2, 15, 0, 1, 500000000, time.UTC)
	server.Observe(domain.CPState{Source: domain.CPSourceProxy, Visible: true, ProbeOK: true, ReadAt: visibleAt})

	readAt := visibleAt.Add(time.Second)
	server.Observe(domain.CPState{Source: domain.CPSourceSelf, Visible: false, ProbeOK: false, ReadAt: readAt})

	api := read(t, client).Status.KubernetesAPI
	if api.CPSource != "self" || api.LastReadOK {
		t.Errorf("section is %+v, want cpSource self and a failed read", api)
	}

	if api.LastReadAt == nil || !api.LastReadAt.Time.Equal(readAt) {
		t.Errorf("lastReadAt is %v, want %v", api.LastReadAt, readAt)
	}

	if api.LastVisibleAt == nil || !api.LastVisibleAt.Time.Equal(visibleAt) {
		t.Errorf("lastVisibleAt is %v, want %v", api.LastVisibleAt, visibleAt)
	}
}

// The view on the local socket has only the node name and the kubernetesAPI
// section, no fields without data.
func TestViewHasOnlyTheRequiredFields(t *testing.T) {
	_, client, _ := start(t)

	resp, err := client.Get("http://agent/")
	if err != nil {
		t.Fatalf("get view: %v", err)
	}
	defer resp.Body.Close()

	var view struct {
		Spec   map[string]json.RawMessage `json:"spec"`
		Status map[string]json.RawMessage `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatalf("decode view: %v", err)
	}

	if got := slices.Sorted(maps.Keys(view.Spec)); !slices.Equal(got, []string{"nodeName"}) {
		t.Errorf("spec fields are %v, want [nodeName]", got)
	}

	if got := slices.Sorted(maps.Keys(view.Status)); !slices.Equal(got, []string{"kubernetesAPI"}) {
		t.Errorf("status fields are %v, want [kubernetesAPI]", got)
	}
}

func TestOnlyGetIsAllowed(t *testing.T) {
	_, client, _ := start(t)

	resp, err := client.Post("http://agent/", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status is %d, want 405", resp.StatusCode)
	}
}

func TestOldSocketFileDoesNotBlockTheStart(t *testing.T) {
	dir, err := os.MkdirTemp("", "fa")
	if err != nil {
		t.Fatalf("make temp dir: %v", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "a.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- New(path, "w1", log.NewNop()).Run(ctx) }()

	for range 200 {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestNoSocketDoesNotStopTheRun(t *testing.T) {
	dir, err := os.MkdirTemp("", "fa")
	if err != nil {
		t.Fatalf("make temp dir: %v", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	// The directory of the socket is missing, as a directory without write access.
	path := filepath.Join(dir, "missing", "a.sock")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- New(path, "w1", log.NewNop()).Run(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("run returned %v before the end of ctx", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}

// The directory of the socket appears or gets write access after the start, as
// when bashible makes it later than the agent starts. The socket starts to
// answer without a restart.
func TestSocketAppearsWhenTheDirectoryIsReady(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, ready func(dir string) error
	}{
		{
			name:   "directory appears",
			before: func(string) error { return nil },
			ready:  func(dir string) error { return os.Mkdir(dir, 0o700) },
		},
		{
			// root ignores the mode: then the first attempt succeeds.
			name:   "directory gets write access",
			before: func(dir string) error { return os.Mkdir(dir, 0o500) },
			ready:  func(dir string) error { return os.Chmod(dir, 0o700) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := os.MkdirTemp("", "fa")
			if err != nil {
				t.Fatalf("make temp dir: %v", err)
			}

			defer func() { _ = os.RemoveAll(base) }()

			dir := filepath.Join(base, "run")
			path := filepath.Join(dir, "a.sock")

			if err := tc.before(dir); err != nil {
				t.Fatalf("prepare socket dir: %v", err)
			}

			server := New(path, "w1", log.NewNop())
			server.retry = 10 * time.Millisecond

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)

			go func() { done <- server.Run(ctx) }()

			time.Sleep(50 * time.Millisecond)

			if err := tc.ready(dir); err != nil {
				t.Fatalf("make socket dir ready: %v", err)
			}

			client := &http.Client{Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", path)
				},
			}}

			waitAccept(path)

			if view := read(t, client); view.Spec.NodeName != "w1" {
				t.Errorf("view is %+v", view)
			}

			cancel()

			if err := <-done; err != nil {
				t.Errorf("run: %v", err)
			}
		})
	}
}
