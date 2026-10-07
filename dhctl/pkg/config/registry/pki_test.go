// Copyright 2025 Flant JSC
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

package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestGeneratePKI(t *testing.T) {
	t.Run("Generate PKI", func(t *testing.T) {
		pki, err := GeneratePKI()
		require.NoError(t, err)

		require.NotNil(t, pki.CA)
	})
}

func TestGetPKI(t *testing.T) {
	t.Run("Get PKI", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()

		err := createInitSecret(ctx, kubeClient)
		require.NoError(t, err)

		pki, err := GetPKI(ctx, kubeClient)
		require.NoError(t, err)

		require.NotNil(t, pki.CA)
	})
}

func TestEnsureInitSecret(t *testing.T) {
	t.Run("creates the secret GetPKI reads, and the namespace it lives in", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()

		require.NoError(t, EnsureInitSecret(ctx, kubeClient, false))

		_, err := kubeClient.CoreV1().Namespaces().Get(ctx, secretsNamespace, metav1.GetOptions{})
		require.NoError(t, err)

		secret, err := kubeClient.CoreV1().Secrets(secretsNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotContains(t, secret.Annotations, initSecretAppliedAnnotation)

		pki, err := GetPKI(ctx, kubeClient)
		require.NoError(t, err)
		require.NotEmpty(t, pki.CA.Cert)
		require.NotEmpty(t, pki.ROUser.Password)
		require.NotEmpty(t, pki.RWUser.Password)
	})

	t.Run("keeps a secret that is already there", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()

		require.NoError(t, createInitSecret(ctx, kubeClient))
		before, err := GetPKI(ctx, kubeClient)
		require.NoError(t, err)

		require.NoError(t, EnsureInitSecret(ctx, kubeClient, false))

		after, err := GetPKI(ctx, kubeClient)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})

	t.Run("marks the secret as applied on a bundle bootstrap", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()

		require.NoError(t, EnsureInitSecret(ctx, kubeClient, true))

		secret, err := kubeClient.CoreV1().Secrets(secretsNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Contains(t, secret.Annotations, initSecretAppliedAnnotation)
	})
}
