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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IndexPath is where a storage replica's syncer answers with its ReplicaIndex, behind the
// kube-rbac-proxy of the storage pod.
const IndexPath = "/images"

// ReplicaIndex is what one replica's store holds, as its syncer last read it. Not a Kubernetes object:
// the answer of the syncer to registry-controller, which merges the replicas into RegistryImage.
type ReplicaIndex struct {
	// Node the replica runs on.
	Node string `json:"node"`

	// Ready is false until the syncer has read its store once after it started. Nothing else is
	// set then.
	Ready bool `json:"ready"`

	// IndexedAt is when the store was read.
	// +optional
	IndexedAt *metav1.Time `json:"indexedAt,omitempty"`

	// SetKnown is false when the syncer could not tell what the cluster declares: every image is then
	// reported outside the set, and no image as missing.
	SetKnown bool `json:"setKnown"`

	// Images are the images the store holds, and those of the set it does not.
	// +optional
	Images []IndexedImage `json:"images,omitempty"`
}

// IndexedImage is one image as one replica holds it.
type IndexedImage struct {
	// Digest of the manifest.
	Digest string `json:"digest"`

	// State of the image on this replica.
	State ImageState `json:"state"`

	// InSet says whether the cluster declares the image.
	InSet bool `json:"inSet"`

	// Owners are every place the image is used. Empty outside the set.
	// +optional
	Owners []ImageOwner `json:"owners,omitempty"`

	// Repositories are where the image is linked from.
	// +optional
	Repositories []string `json:"repositories,omitempty"`

	// MediaType of the manifest.
	// +optional
	MediaType string `json:"mediaType,omitempty"`

	// Size and ReclaimableSize — see RegistryImage. Zero for a missing image.
	// +optional
	Size int64 `json:"size,omitempty"`
	// +optional
	ReclaimableSize int64 `json:"reclaimableSize,omitempty"`

	// MissingBlobs is how many blobs the image names that the store does not hold.
	// +optional
	MissingBlobs int32 `json:"missingBlobs,omitempty"`

	// CorruptedBlobs are the blobs of the image that do not match their digest.
	// +optional
	CorruptedBlobs []string `json:"corruptedBlobs,omitempty"`
}
