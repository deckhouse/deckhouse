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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// releaseGVK is the resource the cluster records its own versions in.
//
// Read as unstructured rather than through the typed API of the Deckhouse controller: this
// binary needs two strings out of it, and taking a dependency on that module's types to get
// them would tie a component of the data plane to the release machinery's Go API.
var releaseGVK = schema.GroupVersionKind{
	Group:   "deckhouse.io",
	Version: "v1alpha1",
	Kind:    "DeckhouseReleaseList",
}

// Phases a release can be in. Only these two matter here: one says what the cluster runs, the
// other what it ran before.
const (
	phaseDeployed   = "Deployed"
	phaseSuperseded = "Superseded"
)

// FromCluster reads what the cluster is running.
//
// The deployed release is the one the collector judges everything against, and the most
// recent superseded one is kept so that a rollback does not have to re-download the release
// it rolls back to.
//
// Returns an error rather than a partial answer when nothing is deployed. The collector needs
// that version to justify any deletion at all, and an empty answer would read as "keep
// nothing".
func FromCluster(ctx context.Context, c client.Client) (Releases, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(releaseGVK)

	if err := c.List(ctx, list); err != nil {
		return Releases{}, fmt.Errorf("listing the cluster's releases: %w", err)
	}

	var releases Releases
	for i := range list.Items {
		item := &list.Items[i]

		version, found, err := unstructured.NestedString(item.Object, "spec", "version")
		if err != nil || !found || version == "" {
			continue
		}
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")

		switch phase {
		case phaseDeployed:
			releases.Deployed = version
		case phaseSuperseded:
			// The most recent of them, which is not the same as the last one listed: the API
			// returns them in name order, and names are versions as strings — where v1.9.0
			// sorts after v1.10.0.
			if newer, err := isNewer(normalize(version), normalize(releases.Previous)); releases.Previous == "" || (err == nil && newer) {
				releases.Previous = version
			}
		}
	}

	if releases.Deployed == "" {
		return releases, fmt.Errorf("no release is deployed, so there is nothing to judge the store against")
	}
	return releases, nil
}
