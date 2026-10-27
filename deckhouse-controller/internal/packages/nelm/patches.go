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

package nelm

import (
	"fmt"

	"github.com/werf/nelm/v2/pkg/resource/spec"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
)

// credentialsPatch adds skip-update to the resource policy of every Secret that
// carries the initial-secret label: such a Secret holds one-time
// credentials the console shows and annotates, so nelm must not revert it.
// The label value is not known up front, so the jq program selects the
// Secrets itself and keeps any policy the chart already sets.
var credentialsPatch = spec.Patch{
	Match: spec.ResourceMatcher{Kinds: []string{"Secret"}},
	Type:  spec.PatchTypeJQ,
	Patch: `if ((.metadata.labels // {}) | has("packages.deckhouse.io/initial-secret"))
then .metadata.annotations["werf.io/resource-policy"] |= (
  if . == null or . == "" then "skip-update"
  elif (split(",") | index("skip-update")) then .
  else . + ",skip-update" end)
else . end`,
}

// renderPatches returns the nelm render patches of the package: its resource
// requests and, for an application, the credentials patch.
func renderPatches(pkg Package) ([]spec.Patch, error) {
	patches, err := resourcesize.Patches(resourceRequests(pkg))
	if err != nil {
		return nil, fmt.Errorf("build resource request patches: %w", err)
	}

	if _, ok := pkg.(application); ok {
		patches = append(patches, credentialsPatch)
	}

	return patches, nil
}

// resourceRequests returns the package's per-workload resource overrides, or nil
// when the feature gate is off or the package carries none.
//
// The gate is read here, at the single point the overrides enter the release
// path, so turning it off leaves the rendered and the installed manifests exactly
// as the chart renders them.
func resourceRequests(pkg Package) []resourcesize.Request {
	if !app.ResourceRequestsEnabled() {
		return nil
	}

	sizer, ok := pkg.(resourceSizer)
	if !ok {
		return nil
	}

	return sizer.GetResourceRequests()
}
