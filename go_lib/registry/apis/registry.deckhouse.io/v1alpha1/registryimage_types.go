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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Labels set on every RegistryImage, for `kubectl get registryimages -l ...`.
const (
	// InSetLabel is "true" for an image a release or a module of the cluster declares, "false" for
	// one the cache holds for no such reason.
	InSetLabel = "registry.deckhouse.io/in-set"

	// StateLabel is the worst state of the image among the replicas — see ImageState.
	StateLabel = "registry.deckhouse.io/state"

	// ModuleLabel is the module the image belongs to. When it belongs to several, the first of them
	// by name; Owners lists them all.
	ModuleLabel = "registry.deckhouse.io/module"

	// OwnerKindLabel is the kind of the image's first owner — see OwnerKind. Absent for an image
	// outside the set.
	OwnerKindLabel = "registry.deckhouse.io/owner-kind"
)

// ImageState is where one image is on one replica.
type ImageState string

const (
	// ImagePresent: the replica holds the image's manifest and every blob it names, as far as the
	// replica knows whole.
	ImagePresent ImageState = "Present"

	// ImageMissing: an image of the set the replica does not hold at all.
	ImageMissing ImageState = "Missing"

	// ImageIncomplete: the replica holds the manifest without some of the blobs it names. What a
	// pull-through cache leaves behind after it was asked for the manifest alone.
	ImageIncomplete ImageState = "Incomplete"

	// ImageCorrupted: a blob of the image does not match its digest. A Repair replaces it; the
	// controller asks for one on its own.
	ImageCorrupted ImageState = "Corrupted"

	// ImageUnknown: the replica did not answer, or has not indexed its store yet.
	ImageUnknown ImageState = "Unknown"
)

// Severity orders states from the best to the worst, for the state an image is summarised by.
func (s ImageState) Severity() int {
	switch s {
	case ImagePresent:
		return 0
	case ImageUnknown:
		return 1
	case ImageIncomplete:
		return 2
	case ImageMissing:
		return 3
	case ImageCorrupted:
		return 4
	}
	return 1
}

// OwnerKind is why an image is in the set.
type OwnerKind string

const (
	// OwnerPlatform: an image of the platform, declared by a Deckhouse release.
	OwnerPlatform OwnerKind = "Platform"

	// OwnerRelease: the image of a Deckhouse release itself, by its version tag.
	OwnerRelease OwnerKind = "Release"

	// OwnerModule: an image of a module installed from a ModuleSource, or the module's package.
	OwnerModule OwnerKind = "Module"

	// OwnerCatalogue: an entry of a catalogue of modules — the release channels of a module the
	// platform reads to know what it can install.
	OwnerCatalogue OwnerKind = "Catalogue"
)

// ImageOwner is one reason an image is in the set.
type ImageOwner struct {
	// Kind is what declares the image.
	Kind OwnerKind `json:"kind"`

	// Module is the module the image belongs to. Empty for the release image.
	// +optional
	Module string `json:"module,omitempty"`

	// Image is the name of the image within the module, as images_digests.json names it.
	// +optional
	Image string `json:"image,omitempty"`

	// Versions are the Deckhouse releases that declare the image (Platform, Release), or the versions
	// of the module (Module), sorted.
	// +optional
	Versions []string `json:"versions,omitempty"`

	// Source is the ModuleSource the module comes from. Empty for the platform's own.
	// +optional
	Source string `json:"source,omitempty"`
}

// ImageReplica is the image on one replica.
type ImageReplica struct {
	// Node the replica runs on.
	Node string `json:"node"`

	// State of the image on this replica.
	State ImageState `json:"state"`

	// Message says what is wrong when the state is not Present: the blobs missing or broken, or why
	// the replica did not answer.
	// +optional
	Message string `json:"message,omitempty"`

	// IndexedAt is when the replica last read its store. The state is as of then.
	// +optional
	IndexedAt *metav1.Time `json:"indexedAt,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryImage is one image of the registry cache, by digest, across its replicas.
//
// Not stored: every request asks each replica what its store holds, and each answers from an index
// it keeps in memory and rebuilds by reading its store. Read-only; an image is repaired with a
// RegistryOperation of type Repair.
type RegistryImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Digest of the image's manifest. The name is the same digest with a dash for the colon.
	Digest string `json:"digest"`

	// InSet says whether a release or a module of the cluster declares the image. An image outside
	// the set was pulled through the cache, or was left by an older release, and the garbage
	// collection removes it.
	InSet bool `json:"inSet"`

	// State is the worst state of the image among the replicas.
	State ImageState `json:"state"`

	// Owners are every place the image is used. Empty outside the set.
	// +optional
	Owners []ImageOwner `json:"owners,omitempty"`

	// Repositories are where in the cache the image is linked from.
	// +optional
	Repositories []string `json:"repositories,omitempty"`

	// MediaType of the manifest.
	// +optional
	MediaType string `json:"mediaType,omitempty"`

	// Size is the full size of the image: its manifest and every blob it names, the manifests of an
	// index included, each counted once.
	//
	// An image is made of layers, and a layer shared by several images is stored once. So the images
	// of the cache add up to more than the cache takes on disk.
	// +optional
	Size *int64 `json:"size,omitempty"`

	// ReclaimableSize is how much the store would shrink if this image alone were removed: the blobs
	// of the image no other image of the store references. Usually close to Size, since the images of
	// the platform share few layers.
	// +optional
	ReclaimableSize *int64 `json:"reclaimableSize,omitempty"`

	// Replicas is the image on each replica of the storage.
	// +optional
	Replicas []ImageReplica `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryImageList is a list of RegistryImage.
type RegistryImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryImage `json:"items"`
}

// NameOf is the object name of the image with this digest: the digest with a dash for the colon,
// because an object name cannot carry one.
func NameOf(digest string) string {
	return strings.Replace(digest, ":", "-", 1)
}

// DigestOf is the digest an object name stands for, and whether it stands for one at all.
func DigestOf(name string) (string, bool) {
	algorithm, encoded, found := strings.Cut(name, "-")
	if !found || algorithm == "" || encoded == "" {
		return "", false
	}
	return algorithm + ":" + encoded, true
}
