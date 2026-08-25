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

package schema

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// TestNodeGroupSpecMatchesLiveCRD runs the same guard against the CRD a running cluster serves,
// rather than the one in this repository. The two can differ: the cluster may run an older release,
// or a newer provider may have widened the schema. Skips unless LIVE_CRD points at a dump:
//
//	kubectl get crd nodegroups.deckhouse.io -o json > /tmp/live_crd.json
//	LIVE_CRD=/tmp/live_crd.json go test ./internal/schema/ -run LiveCRD
func TestNodeGroupSpecMatchesLiveCRD(t *testing.T) {
	path := os.Getenv("LIVE_CRD")
	if path == "" {
		t.Skip("set LIVE_CRD to a dump of the deployed CRD")
	}

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, sigsyaml.Unmarshal(raw, crd))

	var spec *apiextensionsv1.JSONSchemaProps
	for i := range crd.Spec.Versions {
		if !crd.Spec.Versions[i].Storage {
			continue
		}
		stored := crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"]
		spec = &stored
	}
	require.NotNil(t, spec, "live CRD has no storage version")

	goPaths := jsonPaths(reflect.TypeOf(v1.NodeGroupSpec{}), "")

	var missing []string
	walkSchema(spec, "", func(path string) {
		if isAllowed(path) {
			return
		}
		if _, ok := goPaths[path]; !ok {
			missing = append(missing, path)
		}
	})

	sort.Strings(missing)
	require.Empty(t, missing, "live CRD paths absent from v1.NodeGroupSpec:\n%s", strings.Join(missing, "\n"))
}
