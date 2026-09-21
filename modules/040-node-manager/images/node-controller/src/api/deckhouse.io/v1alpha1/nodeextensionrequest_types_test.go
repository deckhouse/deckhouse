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
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// Matched alone is a denominator with no numerator. The markers on the type and
// the shipped CRD are two copies of one declaration — this group ships its CRD
// by hand — so both are pinned here.
func TestNERPrinterColumnsSayWhatTheNodesDidWithIt(t *testing.T) {
	markers := printerColumnMarkers(t, "nodeextensionrequest_types.go")
	require.Equal(t, [][2]string{
		{"Phase", ".status.phase"},
		{"Matched", ".status.matchedNodes"},
		{"Applied", ".status.appliedNodes"},
		{"Failed", ".status.failedNodes"},
		{"Age", ".metadata.creationTimestamp"},
	}, markers)

	paths := testenv.NodeManagerCRDPaths(testenv.NodeExtensionRequestCRDFile)
	require.Len(t, paths, 1)
	raw, err := os.ReadFile(paths[0])
	require.NoError(t, err, "the shipped NodeExtensionRequest CRD must be readable at %s", paths[0])

	var crd struct {
		Spec struct {
			Versions []struct {
				Columns []struct {
					Name     string `json:"name"`
					JSONPath string `json:"jsonPath"`
				} `json:"additionalPrinterColumns"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, sigsyaml.Unmarshal(raw, &crd))
	require.Len(t, crd.Spec.Versions, 1)

	shipped := make([][2]string, 0, len(crd.Spec.Versions[0].Columns))
	for _, column := range crd.Spec.Versions[0].Columns {
		shipped = append(shipped, [2]string{column.Name, column.JSONPath})
	}
	require.Equal(t, markers, shipped)
}
