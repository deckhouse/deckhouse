/*
Copyright 2025 Flant JSC

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

package fencing

import (
	"context"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// leaseNamespace is the namespace where kubelet node leases are stored.
	leaseNamespace = "kube-node-lease"
	// fencingTimeout is how long a lease must be expired before fencing is triggered.
	fencingTimeout = 60 * time.Second
)

// isLeaseExpired checks whether the node lease in kube-node-lease namespace
// has been expired for longer than fencingTimeout.
// Returns (expired, error). If the lease is not found, it is considered expired.
func isLeaseExpired(ctx context.Context, c client.Client, nodeName string) (bool, error) {
	lease := &coordinationv1.Lease{}
	err := c.Get(ctx, types.NamespacedName{
		Namespace: leaseNamespace,
		Name:      nodeName,
	}, lease)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// No lease means the node never renewed — treat as expired.
			return true, nil
		}
		return false, fmt.Errorf("get lease for node %s: %w", nodeName, err)
	}

	if lease.Spec.RenewTime == nil {
		// RenewTime is nil — lease was never renewed.
		return true, nil
	}

	return time.Since(lease.Spec.RenewTime.Time) > fencingTimeout, nil
}
