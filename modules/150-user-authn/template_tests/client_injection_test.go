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

package template_tests

import (
	"sort"

	"github.com/flant/kube-client/manifest/releaseutil"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// injectedClusterRoleBinding is a YAML document appended to a value taken from a DexAuthenticator.
// A line break returns the rendered output to column 0, so unless the value is emitted as a quoted
// scalar the remaining lines become an extra manifest of the release.
const injectedClusterRoleBinding = "\n" +
	"---\n" +
	"apiVersion: rbac.authorization.k8s.io/v1\n" +
	"kind: ClusterRoleBinding\n" +
	"metadata:\n" +
	"  name: injected-by-a-tenant\n" +
	"roleRef:\n" +
	"  apiGroup: rbac.authorization.k8s.io\n" +
	"  kind: ClusterRole\n" +
	"  name: injected-role\n" +
	"subjects: []"

const injectedClusterRoleBindingName = "injected-by-a-tenant"

// hostileValue carries a legitimate-looking value followed by the injected document.
const hostileValue = "tenant@example.test" + injectedClusterRoleBinding

// renderedKinds lists the kind of every manifest of a rendered template file, in file order. It
// splits the stream the way the release does, so a value that stays a single scalar cannot be
// mistaken for a manifest of its own.
func renderedKinds(manifests string) []string {
	split := releaseutil.SplitManifests(manifests)

	names := make([]string, 0, len(split))
	for name := range split {
		names = append(names, name)
	}
	sort.Sort(releaseutil.BySplitManifestsOrder(names))

	kinds := make([]string, 0, len(names))
	for _, name := range names {
		object := ManifestStringToUnstructed(split[name])
		if object == nil {
			continue
		}

		kinds = append(kinds, object.GetKind())
	}

	return kinds
}
