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

package operations

import (
	"testing"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// The configuration admission webhooks select these Secrets by the labels set
// on bootstrap/converge, so edit must keep exactly the same ones.
func TestSecretLabelsForMatchesBootstrapManifests(t *testing.T) {
	for _, secret := range []*corev1.Secret{
		manifests.SecretWithClusterConfig(nil),
		manifests.SecretWithProviderClusterConfig(nil, nil),
		manifests.SecretWithStaticClusterConfig(nil),
	} {
		t.Run(secret.Name, func(t *testing.T) {
			require.Equal(t, secret.Labels, SecretLabelsFor(secret.Name))
		})
	}
}

func TestSecretEditKeepsWebhookLabel(t *testing.T) {
	const (
		namespace = "kube-system"
		secret    = "d8-static-cluster-configuration"
		dataKey   = "static-cluster-configuration.yaml"
	)

	retry.InTestEnvironment = true
	abstractEditing = EditMock

	t.Cleanup(func() { abstractEditing = Edit })

	edit := func(t *testing.T, kubeCl *client.KubernetesClient) *corev1.Secret {
		t.Helper()

		err := SecretEdit(
			t.Context(),
			kubeCl, "static-cluster-configuration", namespace, secret, dataKey,
			SecretLabelsFor(secret),
			nil,
			EditOptions{},
		)
		require.NoError(t, err)

		edited, err := kubeCl.KubeClient.CoreV1().Secrets(namespace).Get(t.Context(), secret, metav1.GetOptions{})
		require.NoError(t, err)

		return edited
	}

	t.Run("absent Secret is created with the webhook label", func(t *testing.T) {
		edited := edit(t, client.NewFakeKubernetesClient())

		require.Equal(t, map[string]string{"name": secret}, edited.Labels)
	})

	t.Run("existing Secret keeps its labels", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		existing := manifests.SecretWithStaticClusterConfig([]byte("apiVersion: deckhouse.io/v1\n"))

		_, err := kubeCl.KubeClient.CoreV1().Secrets(namespace).Create(t.Context(), existing, metav1.CreateOptions{})
		require.NoError(t, err)

		edited := edit(t, kubeCl)

		require.Equal(t, existing.Labels, edited.Labels)
	})
}
