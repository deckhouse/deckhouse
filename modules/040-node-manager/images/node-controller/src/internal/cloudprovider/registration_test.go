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

package cloudprovider

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// repoRoot is relative to this package's directory, which is where `go test` runs:
// <root>/modules/040-node-manager/images/node-controller/src/internal/cloudprovider.
const repoRoot = "../../../../../../.."

// instanceClassCRDPaths are the two places a provider module keeps its InstanceClass CRD,
// relative to the module directory: hand-written providers put it in candi, DVP generates it.
var instanceClassCRDPaths = []string{
	"candi/openapi/instance_class.yaml",
	"crds/instance_class.yaml",
}

var publishedAPIVersionRe = regexp.MustCompile(InstanceClassAPIVersionKey + `:\s*{{\s*b64enc\s+"([^"]+)"`)

// A provider that publishes the wrong version, or none, is not visible in review — it simply stops
// rendering, or worse renders through a conversion webhook whose answer depends on whether the
// webhook is wired yet. Both move the instance-class checksum, and the checksum names an immutable
// MachineTemplate whose rename recreates every node in the NodeGroup.
func TestEveryCloudProviderPublishesInstanceClassAPIVersion(t *testing.T) {
	// Every tree a cloud provider module can live in.
	registrationGlobs := []string{
		"modules/030-cloud-provider-*/templates/registration.yaml",
		"ee/modules/030-cloud-provider-*/templates/registration.yaml",
		"ee/se-plus/modules/030-cloud-provider-*/templates/registration.yaml",
	}

	var paths []string
	for _, glob := range registrationGlobs {
		matches, err := filepath.Glob(filepath.Join(repoRoot, glob))
		require.NoError(t, err)
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		t.Skip("cloud provider modules are not reachable from here; nothing to check")
	}

	for _, path := range paths {
		moduleDir := filepath.Dir(filepath.Dir(path))
		t.Run(filepath.Base(moduleDir), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			content := string(raw)

			if !strings.Contains(content, "instanceClassKind:") {
				t.Skip("provider does not publish an InstanceClass kind")
			}

			match := publishedAPIVersionRe.FindStringSubmatch(content)
			require.NotNil(t, match,
				"%s publishes instanceClassKind, so it must publish %s (the storage version of its "+
					"InstanceClass CRD) next to it", path, InstanceClassAPIVersionKey)

			require.Equal(t, storageVersionOfInstanceClassCRD(t, moduleDir), match[1],
				"%s publishes an InstanceClass version that is not the CRD's storage version", path)

			// node-controller finds registrations by this label, not by the Secret name; both
			// rendered Secrets (the legacy fixed-name one and the per-provider one) must carry it.
			require.GreaterOrEqual(t, strings.Count(content, RegistrationSecretLabel), 2,
				"%s must label both registration Secrets with %s", path, RegistrationSecretLabel)
		})
	}
}

func storageVersionOfInstanceClassCRD(t *testing.T, moduleDir string) string {
	t.Helper()

	var crd struct {
		Spec struct {
			Versions []struct {
				Name    string `json:"name"`
				Storage bool   `json:"storage"`
			} `json:"versions"`
		} `json:"spec"`
	}

	for _, rel := range instanceClassCRDPaths {
		raw, err := os.ReadFile(filepath.Join(moduleDir, rel))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, sigsyaml.Unmarshal(raw, &crd))

		for _, version := range crd.Spec.Versions {
			if version.Storage {
				return version.Name
			}
		}
		require.Fail(t, "no storage version", "%s declares no version with storage: true", rel)
	}

	require.Fail(t, "no InstanceClass CRD", "%s has none of %v", moduleDir, instanceClassCRDPaths)
	return ""
}
