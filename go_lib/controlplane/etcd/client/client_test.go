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
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeClient struct {
	endpoints         []string
	statusErrors      []error
	statusCalls       int
	statusEndpoints   []string
	promotedMemberIDs []uint64
}

func (f *fakeClient) WaitForClusterAvailable(retries int, retryInterval time.Duration) (bool, error) {
	return true, nil
}

func (f *fakeClient) Endpoints() []string {
	return f.endpoints
}

func (f *fakeClient) Status(_ context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	f.statusCalls++
	f.statusEndpoints = append(f.statusEndpoints, endpoint)

	if len(f.statusErrors) == 0 {
		return &clientv3.StatusResponse{}, nil
	}

	err := f.statusErrors[0]
	f.statusErrors = f.statusErrors[1:]
	if err != nil {
		return nil, err
	}

	return &clientv3.StatusResponse{}, nil
}

func (f *fakeClient) MemberAddAsLearner(_ context.Context, _ []string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}

func (f *fakeClient) MemberPromote(_ context.Context, id uint64) (*clientv3.MemberPromoteResponse, error) {
	f.promotedMemberIDs = append(f.promotedMemberIDs, id)
	return &clientv3.MemberPromoteResponse{}, nil
}

func (f *fakeClient) Close() error {
	return nil
}

var _ Interface = (*fakeClient)(nil)

func TestWaitForClusterAvailableSuccess(t *testing.T) {
	cli := &fakeClient{
		endpoints:    []string{"https://127.0.0.1:2379"},
		statusErrors: []error{nil},
	}

	available, err := cli.WaitForClusterAvailable(1, 0)
	if err != nil {
		t.Fatalf("WaitForClusterAvailable() returned error: %v", err)
	}
	if !available {
		t.Fatal("WaitForClusterAvailable() = false, want true")
	}
	if cli.statusCalls != 1 {
		t.Fatalf("Status() calls = %d, want 1", cli.statusCalls)
	}
	if cli.statusEndpoints[0] != cli.endpoints[0] {
		t.Fatalf("Status() endpoint = %q, want %q", cli.statusEndpoints[0], cli.endpoints[0])
	}
}

func TestWaitForClusterAvailableRetriesUntilSuccess(t *testing.T) {
	cli := &fakeClient{
		endpoints:    []string{"https://127.0.0.1:2379"},
		statusErrors: []error{errors.New("temporary failure"), nil},
	}

	available, err := cli.WaitForClusterAvailable(2, 0*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForClusterAvailable() returned error: %v", err)
	}
	if !available {
		t.Fatal("WaitForClusterAvailable() = false, want true")
	}
	if cli.statusCalls != 2 {
		t.Fatalf("Status() calls = %d, want 2", cli.statusCalls)
	}
}

func TestWaitForClusterAvailableTimeout(t *testing.T) {
	cli := &fakeClient{
		endpoints:    []string{"https://127.0.0.1:2379"},
		statusErrors: []error{errors.New("temporary failure"), context.DeadlineExceeded},
	}

	available, err := cli.WaitForClusterAvailable(2, 0)
	if err == nil {
		t.Fatal("WaitForClusterAvailable() error = nil, want timeout error")
	}
	if available {
		t.Fatal("WaitForClusterAvailable() = true, want false")
	}
	if !strings.Contains(err.Error(), "timeout waiting for etcd cluster to be available") {
		t.Fatalf("WaitForClusterAvailable() error = %q, want timeout message", err.Error())
	}
	if cli.statusCalls != 2 {
		t.Fatalf("Status() calls = %d, want 2", cli.statusCalls)
	}
}
