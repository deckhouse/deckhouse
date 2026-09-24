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

// Package node reads Node objects for the controller.
package node

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reader reads one Node at a time, straight from the API server.
//
// It is deliberately built on the uncached reader of the manager. A cached
// client would raise an informer over every Node of the cluster to answer the
// occasional question about a single failed one, which on a large cluster costs
// both the memory of the objects and the watch traffic of the status updates
// every kubelet keeps sending. The reads also have to be current: a Node that
// was deleted and recreated under the same name is told apart by its UID alone,
// and a cache that still holds the previous UID would confirm the reference of
// an incident that belongs to the Node that is gone.
type Reader struct {
	reader client.Reader
}

func NewReader(r client.Reader) *Reader {
	return &Reader{reader: r}
}

// GetNode returns the named Node. A missing Node is reported as an error the
// caller can recognize with apierrors.IsNotFound.
func (r *Reader) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	var node corev1.Node
	if err := r.reader.Get(ctx, types.NamespacedName{Name: name}, &node); err != nil {
		return nil, fmt.Errorf("get node %q: %w", name, err)
	}

	return &node, nil
}
