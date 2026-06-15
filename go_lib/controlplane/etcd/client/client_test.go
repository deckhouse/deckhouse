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

package client

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
)

// TestMain overrides KubernetesAPICallTimeout for the entire test binary so
// that poll loops inside getClusterStatus time out quickly instead of waiting
// the production 1-minute default.
func TestMain(m *testing.M) {
	constants.KubernetesAPICallTimeout = 200 * time.Millisecond
	os.Exit(m.Run())
}

// fakeClient implements Interface and is used as a stub for the inner client
// created by newEtcdClient inside getClusterStatus.
type fakeClient struct {
	endpoints         []string
	statusErr         error
	statusCalls       atomic.Int32
	statusEndpoints   []string
	promotedMemberIDs []uint64
}

func (f *fakeClient) WaitForClusterAvailable(_ int, _ time.Duration) (bool, error) {
	// Not used in unit tests for the real Client; implemented to satisfy Interface.
	return true, nil
}

func (f *fakeClient) Endpoints() []string {
	return f.endpoints
}

func (f *fakeClient) Status(_ context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	f.statusCalls.Add(1)
	f.statusEndpoints = append(f.statusEndpoints, endpoint)
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	return &clientv3.StatusResponse{}, nil
}

func (f *fakeClient) MemberAddAsLearner(_ context.Context, _ string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}

func (f *fakeClient) MemberPromote(_ context.Context, id uint64) (*clientv3.MemberPromoteResponse, error) {
	f.promotedMemberIDs = append(f.promotedMemberIDs, id)
	return &clientv3.MemberPromoteResponse{}, nil
}

func (f *fakeClient) Raw() *clientv3.Client {
	return nil
}

func (f *fakeClient) Close() error {
	return nil
}

var _ Interface = (*fakeClient)(nil)

// newTestClient creates a *Client with endpointsOverride and a newEtcdClient
// factory that always returns the provided inner Interface. This allows testing
// the real WaitForClusterAvailable / getClusterStatus logic without a live etcd.
func newTestClient(endpoints []string, factory func([]string) (Interface, error)) *Client {
	return &Client{
		endpointsOverride: endpoints,
		newEtcdClient:     factory,
	}
}

// TestWaitForClusterAvailableSuccess verifies that WaitForClusterAvailable
// returns true immediately when all endpoints respond without error.
func TestWaitForClusterAvailableSuccess(t *testing.T) {
	ep := "https://127.0.0.1:2379"
	inner := &fakeClient{}
	cli := newTestClient([]string{ep}, func(_ []string) (Interface, error) {
		return inner, nil
	})

	available, err := cli.WaitForClusterAvailable(1, 0)
	if err != nil {
		t.Fatalf("WaitForClusterAvailable() returned error: %v", err)
	}
	if !available {
		t.Fatal("WaitForClusterAvailable() = false, want true")
	}
	if inner.statusCalls.Load() < 1 {
		t.Fatalf("Status() was not called")
	}
}

// TestWaitForClusterAvailableMultipleEndpoints verifies that all endpoints
// are checked, not just the first one.
func TestWaitForClusterAvailableMultipleEndpoints(t *testing.T) {
	eps := []string{"https://10.0.0.1:2379", "https://10.0.0.2:2379", "https://10.0.0.3:2379"}
	inner := &fakeClient{}
	cli := newTestClient(eps, func(_ []string) (Interface, error) {
		return inner, nil
	})

	available, err := cli.WaitForClusterAvailable(1, 0)
	if err != nil {
		t.Fatalf("WaitForClusterAvailable() returned error: %v", err)
	}
	if !available {
		t.Fatal("WaitForClusterAvailable() = false, want true")
	}
	// Status must be called once per endpoint (one newEtcdClient call per endpoint).
	if got := int(inner.statusCalls.Load()); got != len(eps) {
		t.Fatalf("Status() calls = %d, want %d (one per endpoint)", got, len(eps))
	}
}

// TestWaitForClusterAvailableRetriesUntilSuccess verifies that the outer retry
// loop retries when getClusterStatus fails and eventually succeeds.
func TestWaitForClusterAvailableRetriesUntilSuccess(t *testing.T) {
	ep := "https://127.0.0.1:2379"
	var callCount atomic.Int32
	cli := newTestClient([]string{ep}, func(_ []string) (Interface, error) {
		if callCount.Add(1) == 1 {
			return nil, errors.New("temporary failure")
		}
		return &fakeClient{}, nil
	})

	available, err := cli.WaitForClusterAvailable(2, 0*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForClusterAvailable() returned error: %v", err)
	}
	if !available {
		t.Fatal("WaitForClusterAvailable() = false, want true")
	}
}

// TestWaitForClusterAvailableExhaustsRetries verifies that WaitForClusterAvailable
// returns an error after all retries are exhausted.
func TestWaitForClusterAvailableExhaustsRetries(t *testing.T) {
	ep := "https://127.0.0.1:2379"
	cli := newTestClient([]string{ep}, func(_ []string) (Interface, error) {
		return nil, errors.New("persistent failure")
	})

	available, err := cli.WaitForClusterAvailable(3, 0)
	if err == nil {
		t.Fatal("WaitForClusterAvailable() error = nil, want timeout error")
	}
	if available {
		t.Fatal("WaitForClusterAvailable() = true, want false")
	}
	if !strings.Contains(err.Error(), "timeout waiting for etcd cluster to be available") {
		t.Fatalf("WaitForClusterAvailable() error = %q, want timeout message", err.Error())
	}
}
