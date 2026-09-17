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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	libdhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
)

func TestSplitNodeCustomizationsTakesOnlyNodeConfigs(t *testing.T) {
	resources := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static

---

apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  network:
    interfaces:
    - name: eno1
      dhcp: true
`
	customizations, rest := splitNodeCustomizations(resources)

	require.Len(t, customizations, 1)
	require.Contains(t, customizations[0], "master-0")
	require.Contains(t, rest, "kind: NodeGroup")
	require.NotContains(t, rest, "kind: NodeConfig")
}

func TestSplitNodeCustomizationsKeepsEverythingElseInRest(t *testing.T) {
	resources := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker

---

not-a-kubernetes-object: true

---

apiVersion: deckhouse.io/v1
kind: NodeConfig
metadata: {name: decoy}

---

apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0

---

apiVersion: v1
kind: Secret
metadata:
  name: creds
`
	customizations, rest := splitNodeCustomizations(resources)

	require.Len(t, customizations, 1)
	require.Contains(t, customizations[0], "master-0")

	// Everything the split does not claim comes back as a valid multi-document
	// YAML, in order, so the resources phase still sees it whole. The decoy
	// shares the kind but not the group, and stays.
	require.Equal(t, []string{
		"apiVersion: deckhouse.io/v1\nkind: NodeGroup\nmetadata:\n  name: worker",
		"not-a-kubernetes-object: true",
		"apiVersion: deckhouse.io/v1\nkind: NodeConfig\nmetadata: {name: decoy}",
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds",
	}, libdhctlyaml.SplitYAML(rest))
}

func TestSplitNodeCustomizationsLeavesNoResourcesWhenAllDocumentsAreOurs(t *testing.T) {
	resources := `
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0

---

apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-1
`
	customizations, rest := splitNodeCustomizations(resources)

	require.Len(t, customizations, 2)
	// The resources phase runs on ResourcesYAML != "" (cluster-bootstrapper.go),
	// so taking every document must leave an empty string, not a bare "---".
	require.Empty(t, rest)
}
