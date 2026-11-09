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

package gc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestALostLeaseWaitsForTheWorkItStarted: leader election returns as soon as the lease is lost and
// leaves the work it started running. Returned there, the caller read the outcome while the work still
// wrote it, and the next replica collected beside a deletion this one had not stopped.
func TestALostLeaseWaitsForTheWorkItStarted(t *testing.T) {
	clientset := fake.NewClientset()
	lease := &ClusterLease{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client:    clientset.CoordinationV1(),
		Namespace: "d8-system", Name: "registry-storage-gc", Identity: "master-0",
		LeaseDuration: time.Second, RenewDeadline: 500 * time.Millisecond, RetryPeriod: 100 * time.Millisecond,
	}

	var finished, refusing atomic.Bool
	// The API server stops taking the renewals once the work is under way, as when it is too slow to
	// answer within RenewDeadline: the lease is lost with the work still running.
	clientset.PrependReactor("update", "leases", func(k8stesting.Action) (bool, runtime.Object, error) {
		if refusing.Load() {
			return true, nil, errors.New("the API server did not answer")
		}
		return false, nil, nil
	})

	err := lease.Hold(context.Background(), func(ctx context.Context) error {
		refusing.Store(true)
		<-ctx.Done()
		// The deletion in flight finishes after the lease is gone.
		time.Sleep(100 * time.Millisecond)
		finished.Store(true)
		return ctx.Err()
	})

	assert.True(t, finished.Load(), "Hold returned before the work it started had finished")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lease was lost while collecting")
}
