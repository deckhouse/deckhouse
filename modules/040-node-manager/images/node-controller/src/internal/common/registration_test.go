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

package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRoot is relative to this package's directory, which is where `go test` runs:
// <root>/modules/040-node-manager/images/node-controller/src/internal/common.
const repoRoot = "../../../../../../.."

// registrationGlobs cover every tree a cloud provider module can live in.
var registrationGlobs = []string{
	"modules/030-cloud-provider-*/templates/registration.yaml",
	"ee/modules/030-cloud-provider-*/templates/registration.yaml",
	"ee/se-plus/modules/030-cloud-provider-*/templates/registration.yaml",
}

// A cloud provider that publishes instanceClassKind without instanceClassAPIVersion leaves
// node-controller unable to read the InstanceClass, and node-controller is deliberately unable to
// work the version out for itself: the value decides the instance-class checksum, the checksum
// names an immutable MachineTemplate, and a template rename recreates every node in the NodeGroup.
// The failure is silent in review — the provider simply stops rendering — so it is caught here.
func TestEveryCloudProviderPublishesInstanceClassAPIVersion(t *testing.T) {
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
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path))), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			content := string(raw)

			if !strings.Contains(content, "instanceClassKind:") {
				t.Skip("provider does not publish an InstanceClass kind")
			}
			require.Contains(t, content, InstanceClassAPIVersionKey+":",
				"%s publishes instanceClassKind, so it must publish %s (the storage version of its "+
					"InstanceClass CRD) next to it", path, InstanceClassAPIVersionKey)
		})
	}
}
