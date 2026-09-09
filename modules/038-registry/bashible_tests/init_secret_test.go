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
	"testing"

	"github.com/stretchr/testify/require"
)

const initSecretStep = "cluster-bootstrap/073_init_registry_secrets.sh.tpl"

// TestThePKIReachesTheClusterOnEveryPath is the failure this test exists for, measured on a cluster:
// an installation from a bundle filled its store, brought up the control plane, and then died with
//
//	create Deckhouse manifests: create deckhouse registry secret data: get PKI:
//	get secret 'd8-system/registry-init': secrets "registry-init" not found
//
// The installer generates this PKI afresh for every bashible bundle and keeps no copy, so the secret
// the node uploads is the only place it survives. Whatever else this step is gated on, the upload
// itself must happen.
func TestThePKIReachesTheClusterOnEveryPath(t *testing.T) {
	for name, registry := range map[string]map[string]any{
		"from a bundle":     bundleRegistry(),
		"legacy Local mode": localRegistry(),
	} {
		t.Run(name, func(t *testing.T) {
			body := render(t, initSecretStep, registry)

			require.Contains(t, body, `/api/v1/namespaces/d8-system/secrets`,
				"the PKI must be uploaded: dhctl reads it back and cannot regenerate it")
			require.Contains(t, body, `"name":"registry-init"`)
		})
	}
}

// TestABundleInstallationDoesNotStartTheLegacyStateMachine is the other half, and the reason the
// upload was ever gated: unapplied, this secret makes the previous implementation record a target
// mode and wait for a DaemonSet that a bundle-installed cluster has scaled to zero. The current
// implementation then refuses to take over from a predecessor that reads as mid-transition, so which
// of the two writes its state first decides whether the cluster has a registry at all.
func TestABundleInstallationDoesNotStartTheLegacyStateMachine(t *testing.T) {
	body := render(t, initSecretStep, bundleRegistry())

	require.Contains(t, body, `INIT_ANNOTATIONS='{"registry.deckhouse.io/is-applied":""}'`,
		"the secret must arrive already consumed, or the legacy state machine starts on it")
	require.Contains(t, body, `--argjson annotations "$INIT_ANNOTATIONS"`,
		"the annotation must reach the secret that is actually posted")
}

// TestALiveClusterIsUnaffected: `fromBundle` is absent everywhere except an installation from a
// bundle, and for everything else this step must behave exactly as it did before — the previous
// implementation's own initialization depends on this secret arriving unapplied.
func TestALiveClusterIsUnaffected(t *testing.T) {
	body := render(t, initSecretStep, localRegistry())

	require.Contains(t, body, `INIT_ANNOTATIONS='{}'`,
		"annotating the secret in a legacy cluster would silently skip that cluster's own registry initialization")
	require.NotContains(t, body, "is-applied")
}
