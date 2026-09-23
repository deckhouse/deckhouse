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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"fencing-agent/internal/domain"
)

func unlabeledNode(name string, addresses ...corev1.NodeAddress) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID("uid-" + name),
		},
		Status: corev1.NodeStatus{Addresses: addresses},
	}
}

func TestGetNodeReturnsIdentityGroupAndInternalIP(t *testing.T) {
	client := fake.NewClientset(objects(
		node("worker-1", "worker",
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
			internal("10.0.0.2"),
		),
		node("worker-2", "", internal("10.0.0.3")),
		unlabeledNode("worker-3", internal("10.0.0.4")),
	)...)
	nodes := NewNodes(client)

	tests := []struct {
		name string
		want domain.NodeRecord
	}{
		{
			name: "worker-1",
			want: domain.NodeRecord{Name: "worker-1", UID: "uid-worker-1", IP: "10.0.0.2", NodeGroup: "worker"},
		},
		{
			name: "worker-2",
			want: domain.NodeRecord{Name: "worker-2", UID: "uid-worker-2", IP: "10.0.0.3", NodeGroup: ""},
		},
		{
			name: "worker-3",
			want: domain.NodeRecord{Name: "worker-3", UID: "uid-worker-3", IP: "10.0.0.4", NodeGroup: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nodes.GetNode(t.Context(), tt.name)
			if err != nil {
				t.Fatalf("GetNode(%q) error = %v", tt.name, err)
			}

			if got != tt.want {
				t.Fatalf("GetNode(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetNodeIsAConsistentGet(t *testing.T) {
	client := fake.NewClientset(objects(node("worker-1", "worker", internal("10.0.0.2")))...)

	seen := 0
	client.PrependReactor("get", "nodes", func(a clienttesting.Action) (bool, runtime.Object, error) {
		seen++

		get, ok := a.(clienttesting.GetActionImpl)
		if !ok {
			t.Errorf("get action type = %T, want clienttesting.GetActionImpl", a)
			return false, nil, nil
		}

		if get.GetOptions.ResourceVersion != "" {
			t.Errorf("GetOptions.ResourceVersion = %q, want empty (consistent read)", get.GetOptions.ResourceVersion)
		}

		return false, nil, nil
	})

	if _, err := NewNodes(client).GetNode(t.Context(), "worker-1"); err != nil {
		t.Fatalf("GetNode error = %v", err)
	}

	if seen != 1 {
		t.Fatalf("get reactor calls = %d, want 1", seen)
	}

	gets, lists := 0, 0
	for _, action := range client.Actions() {
		switch action.GetVerb() {
		case "get":
			gets++
		case "list":
			lists++
		}
	}

	if gets != 1 || lists != 0 {
		t.Fatalf("actions: get = %d, list = %d, want get = 1, list = 0", gets, lists)
	}
}

func TestGetNodeKeepsNotFoundRecognizable(t *testing.T) {
	client := fake.NewClientset(objects(node("worker-1", "worker", internal("10.0.0.2")))...)

	got, err := NewNodes(client).GetNode(t.Context(), "worker-9")
	if err == nil {
		t.Fatal("GetNode of a missing Node returned no error")
	}

	if !errors.Is(err, domain.ErrNodeNotFound) {
		t.Errorf("errors.Is(err, domain.ErrNodeNotFound) = false for %v", err)
	}

	if !apierrors.IsNotFound(err) {
		t.Errorf("apierrors.IsNotFound(err) = false for %v", err)
	}

	if got != (domain.NodeRecord{}) {
		t.Errorf("GetNode record = %+v, want zero NodeRecord", got)
	}
}

func TestGetNodeDoesNotDisguiseTransportErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantDeadline bool
	}{
		{name: "deadline exceeded", err: context.DeadlineExceeded, wantDeadline: true},
		{name: "connection refused", err: errors.New("dial tcp 10.0.0.1:6443: connect: connection refused")},
		{name: "service unavailable", err: apierrors.NewServiceUnavailable("etcd is unavailable")},
		{
			name: "forbidden",
			err:  apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "worker-1", errors.New("rbac denied")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(objects(node("worker-1", "worker", internal("10.0.0.2")))...)
			client.PrependReactor("get", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.err
			})

			got, err := NewNodes(client).GetNode(t.Context(), "worker-1")
			if err == nil {
				t.Fatal("GetNode returned no error")
			}

			if errors.Is(err, domain.ErrNodeNotFound) {
				t.Errorf("errors.Is(err, domain.ErrNodeNotFound) = true for %v", err)
			}

			if apierrors.IsNotFound(err) {
				t.Errorf("apierrors.IsNotFound(err) = true for %v", err)
			}

			if tt.wantDeadline && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("errors.Is(err, context.DeadlineExceeded) = false for %v", err)
			}

			if got != (domain.NodeRecord{}) {
				t.Errorf("GetNode record = %+v, want zero NodeRecord", got)
			}
		})
	}
}

func TestGetNodeHonoursTheCallerDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	client, err := New(&rest.Config{Host: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = NewNodes(client).GetNode(ctx, "worker-1")
	elapsed := time.Since(start)

	if elapsed >= time.Second {
		t.Errorf("GetNode returned after %s, want < 1s with a 50ms caller deadline", elapsed)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false for %v", err)
	}
}

func TestGetNodePicksTheFirstInternalIP(t *testing.T) {
	client := fake.NewClientset(objects(
		node("worker-1", "worker",
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
			internal("10.0.0.2"),
			internal("fd00::2"),
		),
	)...)

	record, err := NewNodes(client).GetNode(t.Context(), "worker-1")
	if err != nil {
		t.Fatalf("GetNode error = %v", err)
	}

	if record.IP != "10.0.0.2" {
		t.Errorf("GetNode IP = %q, want %q", record.IP, "10.0.0.2")
	}
}
