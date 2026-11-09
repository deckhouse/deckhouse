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

// TestThePKIReachesTheClusterOnEveryPath is the failure this test exists for: an installation from a
// bundle fills its store, brings up the control plane, and then dies with
//
//	create Deckhouse manifests: create deckhouse registry secret data: get PKI:
//	get secret 'd8-system/registry-init': secrets "registry-init" not found
//
// The installer generates this PKI afresh for every bashible bundle and keeps no copy, so the secret
// the node uploads is the only place it survives. The step must upload it whenever it renders.
func TestThePKIReachesTheClusterOnEveryPath(t *testing.T) {
	body := render(t, initSecretStep, bundleRegistry())

	require.Contains(t, body, `/api/v1/namespaces/d8-system/secrets`,
		"the PKI must be uploaded: dhctl reads it back and cannot regenerate it")
	require.Contains(t, body, `"name":"registry-init"`)
}
