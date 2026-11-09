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

package bashible_tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestTheBootstrapRegistryServesOnTheStorePort pins the port the registry an installation from a
// bundle starts on the first master shares with the cluster's store.
//
// Shared on purpose. The two serve the same /opt/deckhouse/registry/local_data, and the one thing
// that keeps the store from starting while the bootstrap registry still serves is that both declare
// the same host port: until dhctl removes the static pod, the scheduler has nowhere to place the
// store on that master. Moved apart — the store to one port, the bootstrap registry left on another —
// the store's pod is scheduled at once and its syncer starts filling and collecting a directory the
// bootstrap registry is still serving from.
//
// Every place the bootstrap registry's port is spelled out is checked: where it listens, the token
// realm it hands out, the liveness probe that waits for it, the fill that writes into it, and the
// host port its static pod declares.
func TestTheBootstrapRegistryServesOnTheStorePort(t *testing.T) {
	address := fmt.Sprintf("${discovered_node_ip}:%d", registry_const.StoragePort)
	legacy := fmt.Sprintf("${discovered_node_ip}:%d", registry_const.LegacyStoragePort)

	t.Run("the process the bootstrap starts first", func(t *testing.T) {
		body := effective(render(t, "cluster-bootstrap/020_install_registry_igniter.sh.tpl", bundleRegistry()))

		require.Contains(t, body, fmt.Sprintf(`addr: "%s"`, address))
		require.Contains(t, body, fmt.Sprintf("realm: https://%s/auth/token", address))
		require.Contains(t, body, fmt.Sprintf(`liveness_probe "https://%s"`, address))
		require.NotContains(t, body, legacy)
	})

	t.Run("the fill from the bundle", func(t *testing.T) {
		body := effective(render(t, "cluster-bootstrap/021_fill_registry_igniter.sh.tpl", bundleRegistry()))

		require.Contains(t, body, fmt.Sprintf(`address: "%s"`, address))
		require.NotContains(t, body, legacy)
	})

	t.Run("the static pod the store takes over from", func(t *testing.T) {
		body := effective(render(t, "cluster-bootstrap/070_install_registry.sh.tpl", bundleRegistry()))

		require.Contains(t, body, fmt.Sprintf(`addr: "%s"`, address))
		require.Contains(t, body, fmt.Sprintf("realm: https://%s/auth/token", address))
		require.Contains(t, body, fmt.Sprintf("containerPort: %d\nhostPort: %d",
			registry_const.StoragePort, registry_const.StoragePort),
			"the static pod has to declare the port the store's pod declares, or the scheduler places the store beside it")
		// The token service's port, which the store's pod declares as well — the second thing they
		// share, and the one that still separates the store from the previous implementation's static
		// pod of the same name.
		require.Contains(t, body, "name: auth\ncontainerPort: 5051")
		require.NotContains(t, body, legacy)
	})
}
