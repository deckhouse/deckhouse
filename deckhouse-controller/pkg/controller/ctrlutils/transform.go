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

package ctrlutils

import (
	corev1 "k8s.io/api/core/v1"
)

// TransformStripNode is the cache transform for Nodes. The manager cache holds every node of
// the cluster, and most of a Node is what deckhouse-controller never reads: the managed fields,
// the image list, the conditions and the volume lists. They are dropped before the object is
// committed to the cache; the metadata, the spec and the rest of the status stay. Code that
// needs a dropped field reads the node from the API, not from the cache.
//
// A field is written only when there is something to drop, which keeps the transform idempotent
// and free of writes for an object that comes through again (see
// TransformStripManagedFields in controller-runtime). Anything else, a deletion tombstone
// included, passes as is.
func TransformStripNode(obj any) (any, error) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return obj, nil
	}

	if node.ManagedFields != nil {
		node.ManagedFields = nil
	}
	if node.Status.Images != nil {
		node.Status.Images = nil
	}
	if node.Status.Conditions != nil {
		node.Status.Conditions = nil
	}
	if node.Status.VolumesInUse != nil {
		node.Status.VolumesInUse = nil
	}
	if node.Status.VolumesAttached != nil {
		node.Status.VolumesAttached = nil
	}

	return node, nil
}
