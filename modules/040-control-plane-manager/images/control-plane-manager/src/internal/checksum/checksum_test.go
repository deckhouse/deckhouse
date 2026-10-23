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

package checksum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const revocationWebhookConfigKey = "extra-file-revocation-webhook-config.yaml"

// The extra files of a component are what is written to disk next to its manifest. The
// authorization config of kube-apiserver names the kubeconfig of every webhook authorizer, and
// kube-apiserver does not start when one of them is missing, so each of those kubeconfigs must be
// an extra file of kube-apiserver.
func TestExtraFileKeysForPodComponent(t *testing.T) {
	keys := ExtraFileKeysForPodComponent("kube-apiserver")
	for _, key := range []string{
		"extra-file-authorization-config.yaml",
		"extra-file-webhook-config.yaml",
		revocationWebhookConfigKey,
	} {
		assert.Contains(t, keys, key)
	}
	assert.NotContains(t, keys, "kube-apiserver.yaml.tpl", "the manifest template is not an extra file")

	for _, component := range []string{"etcd", "kube-controller-manager", "kube-scheduler"} {
		assert.NotContains(t, ExtraFileKeysForPodComponent(component), revocationWebhookConfigKey, component)
	}
	assert.Nil(t, ExtraFileKeysForPodComponent("unknown"))
}

func TestComponentChecksumFollowsTheRevocationWebhookConfig(t *testing.T) {
	secretData := map[string][]byte{
		"kube-apiserver.yaml.tpl":              []byte("manifest"),
		"kube-scheduler.yaml.tpl":              []byte("scheduler manifest"),
		"extra-file-authorization-config.yaml": []byte("authorizers"),
		"extra-file-webhook-config.yaml":       []byte("server: https://127.0.0.1:40443/"),
	}
	checksums := func() (string, string) {
		t.Helper()
		apiserver, err := ComponentChecksum(secretData, "kube-apiserver")
		require.NoError(t, err)
		scheduler, err := ComponentChecksum(secretData, "kube-scheduler")
		require.NoError(t, err)
		return apiserver, scheduler
	}

	withoutRevocation, schedulerBefore := checksums()

	secretData[revocationWebhookConfigKey] = []byte("server: https://127.0.0.1:40443/revocation")
	withRevocation, schedulerAfter := checksums()
	assert.NotEqual(t, withoutRevocation, withRevocation, "adding the revocation webhook must restart kube-apiserver")
	assert.Equal(t, schedulerBefore, schedulerAfter, "the revocation webhook is not a dependency of kube-scheduler")

	secretData[revocationWebhookConfigKey] = []byte("server: https://127.0.0.1:40444/revocation")
	changedRevocation, _ := checksums()
	assert.NotEqual(t, withRevocation, changedRevocation, "a changed revocation webhook config must restart kube-apiserver")
}
