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

// Package images assembles the RegistryImage resource: it asks every storage replica for the index of
// its store and merges the answers, image by image.
package images

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

// Answer is one replica's answer: its index, or why there is none.
type Answer struct {
	// Node the replica runs on.
	Node string

	// Index is what the replica answered. Nil when Err is set.
	Index *registryimagev1alpha1.ReplicaIndex

	// Err is why the replica did not answer.
	Err error
}

// Merge turns the replicas' answers into one RegistryImage per digest, sorted by name.
//
// An image a replica answered about is on it as the replica says. One the replica holds nothing of
// and that is outside the set is not on it, and the replica is left out of the image's replicas. A
// replica that did not answer, or has not read its store yet, is Unknown for every image.
func Merge(answers []Answer) []registryimagev1alpha1.RegistryImage {
	byDigest := map[string]*registryimagev1alpha1.RegistryImage{}
	held := map[string]map[string]bool{}

	get := func(digest string) *registryimagev1alpha1.RegistryImage {
		image := byDigest[digest]
		if image == nil {
			image = &registryimagev1alpha1.RegistryImage{
				TypeMeta: metav1.TypeMeta{
					APIVersion: registryimagev1alpha1.GroupVersion.String(),
					Kind:       registryimagev1alpha1.RegistryImageKind,
				},
				ObjectMeta: metav1.ObjectMeta{Name: registryimagev1alpha1.NameOf(digest)},
				Digest:     digest,
			}
			byDigest[digest] = image
			held[digest] = map[string]bool{}
		}
		return image
	}

	var silent []Answer
	for _, answer := range answers {
		if answer.Err != nil || answer.Index == nil || !answer.Index.Ready {
			silent = append(silent, answer)
			continue
		}
		for i := range answer.Index.Images {
			indexed := &answer.Index.Images[i]
			image := get(indexed.Digest)
			held[indexed.Digest][answer.Node] = true

			image.InSet = image.InSet || indexed.InSet
			image.Owners = mergeOwners(image.Owners, indexed.Owners)
			image.Repositories = union(image.Repositories, indexed.Repositories)
			if image.MediaType == "" {
				image.MediaType = indexed.MediaType
			}
			// The size of a copy that holds the whole image, which every whole copy agrees on.
			if indexed.State != registryimagev1alpha1.ImageMissing && indexed.MissingBlobs == 0 && image.Size == nil {
				size, reclaimable := indexed.Size, indexed.ReclaimableSize
				image.Size, image.ReclaimableSize = &size, &reclaimable
			}

			image.Replicas = append(image.Replicas, registryimagev1alpha1.ImageReplica{
				Node:      answer.Node,
				State:     indexed.State,
				Message:   stateMessage(indexed),
				IndexedAt: answer.Index.IndexedAt,
			})
		}
	}

	images := make([]registryimagev1alpha1.RegistryImage, 0, len(byDigest))
	for digest, image := range byDigest {
		// A replica that answered and did not name an image of the set is missing it — it can say so
		// only of what its own set declares, and the replicas declare the same set.
		for _, answer := range answers {
			if held[digest][answer.Node] || answer.Err != nil || answer.Index == nil || !answer.Index.Ready {
				continue
			}
			if image.InSet && answer.Index.SetKnown {
				image.Replicas = append(image.Replicas, registryimagev1alpha1.ImageReplica{
					Node: answer.Node, State: registryimagev1alpha1.ImageMissing, IndexedAt: answer.Index.IndexedAt,
				})
			}
		}
		for _, answer := range silent {
			image.Replicas = append(image.Replicas, registryimagev1alpha1.ImageReplica{
				Node: answer.Node, State: registryimagev1alpha1.ImageUnknown, Message: SilentMessage(answer),
			})
		}
		sort.Slice(image.Replicas, func(i, j int) bool { return image.Replicas[i].Node < image.Replicas[j].Node })

		image.State = worst(image.Replicas)
		image.Labels = labelsOf(image)
		images = append(images, *image)
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Name < images[j].Name })
	return images
}

// worst is the state an image is summarised by: the worst among its replicas.
func worst(replicas []registryimagev1alpha1.ImageReplica) registryimagev1alpha1.ImageState {
	state := registryimagev1alpha1.ImagePresent
	if len(replicas) == 0 {
		return registryimagev1alpha1.ImageUnknown
	}
	for _, replica := range replicas {
		if replica.State.Severity() > state.Severity() {
			state = replica.State
		}
	}
	return state
}

// labelsOf is what an image can be selected by.
func labelsOf(image *registryimagev1alpha1.RegistryImage) map[string]string {
	labels := map[string]string{
		registryimagev1alpha1.InSetLabel: fmt.Sprint(image.InSet),
		registryimagev1alpha1.StateLabel: string(image.State),
	}
	if len(image.Owners) > 0 {
		labels[registryimagev1alpha1.OwnerKindLabel] = string(image.Owners[0].Kind)
	}
	var modules []string
	for _, owner := range image.Owners {
		if owner.Module != "" {
			modules = append(modules, owner.Module)
		}
	}
	if len(modules) > 0 {
		sort.Strings(modules)
		labels[registryimagev1alpha1.ModuleLabel] = LabelValue(modules[0])
	}
	return labels
}

// LabelValue keeps a value within what a label allows: 63 characters of letters, digits, dashes,
// underscores and dots, beginning and ending with a letter or a digit.
func LabelValue(value string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '-'
	}, value)
	if len(mapped) > 63 {
		mapped = mapped[:63]
	}
	return strings.Trim(mapped, "-_.")
}

// stateMessage says what is wrong with an image on a replica.
func stateMessage(image *registryimagev1alpha1.IndexedImage) string {
	switch image.State {
	case registryimagev1alpha1.ImageCorrupted:
		return "Blobs that do not match their digests: " + strings.Join(image.CorruptedBlobs, ", ") + "."
	case registryimagev1alpha1.ImageIncomplete:
		return fmt.Sprintf("%d blobs the image names are not in the store.", image.MissingBlobs)
	}
	return ""
}

// SilentMessage says why a replica has no index to give.
func SilentMessage(answer Answer) string {
	if answer.Err != nil {
		return "The replica did not answer: " + answer.Err.Error()
	}
	return "The replica has not read its store since it started."
}

func union(into, more []string) []string {
	for _, value := range more {
		if !contains(into, value) {
			into = append(into, value)
		}
	}
	sort.Strings(into)
	return into
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// mergeOwners adds the owners one replica reports to those of the others. The replicas read the same
// set, so they mostly agree; the versions are merged where they do not.
func mergeOwners(into, more []registryimagev1alpha1.ImageOwner) []registryimagev1alpha1.ImageOwner {
	for _, owner := range more {
		merged := false
		for i := range into {
			existing := &into[i]
			if existing.Kind == owner.Kind && existing.Module == owner.Module &&
				existing.Image == owner.Image && existing.Source == owner.Source {
				existing.Versions = union(existing.Versions, owner.Versions)
				merged = true
				break
			}
		}
		if !merged {
			owner.Versions = union(nil, owner.Versions)
			into = append(into, owner)
		}
	}
	return into
}
