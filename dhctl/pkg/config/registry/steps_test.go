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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func createInitSecret(ctx context.Context, kubeClient client.KubeClient, applied bool) error {
	pki, err := GeneratePKI()
	if err != nil {
		return err
	}

	pkiYaml, err := yaml.Marshal(pki)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      initSecretName,
			Namespace: secretsNamespace,
		},
		Data: map[string][]byte{
			"config": pkiYaml,
		},
	}

	if applied {
		secret.Annotations = map[string]string{
			initSecretAppliedAnnotation: "",
		}
	}

	return createOrUpdateSecret(ctx, kubeClient, secret)
}

func createOrUpdateSecret(ctx context.Context, kubeClient client.KubeClient, secret *corev1.Secret) error {
	_, err := kubeClient.
		CoreV1().
		Secrets(secret.Namespace).
		Create(ctx, secret, metav1.CreateOptions{})

	if err != nil && apierrors.IsAlreadyExists(err) {
		_, err = kubeClient.
			CoreV1().
			Secrets(secret.Namespace).
			Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

func TestCheckRegistryInitializationNonLegacyNoStateSecret(t *testing.T) {
	// registry-init applied, registry-state ABSENT: new-arch readiness no longer
	// needs registry-state (cache/agent readiness is checked earlier by dhctl).
	ctx := t.Context()
	kubeClient := client.NewFakeKubernetesClient()

	require.NoError(t, createInitSecret(ctx, kubeClient, true))

	cfg := ConfigBuilder()

	err := checkRegistryInitialization(ctx, kubeClient, cfg)
	if err != nil {
		t.Fatalf("non-legacy readiness with applied init + no state must pass, got %v", err)
	}
}

func TestCheckRegistryInitialization(t *testing.T) {
	t.Run("legacy - should delete init secret", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder(
			WithLegacyMode(),
		)

		// Setup: create non-applied init secret
		err := createInitSecret(ctx, kubeClient, false)
		require.NoError(t, err)

		isExist, isApplied, err := getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)

		require.True(t, isExist, "Init secret should exist initially")
		require.False(t, isApplied, "Init secret should not be applied initially")

		// First run: delete the secret
		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.NoError(t, err)

		isExist, _, err = getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)

		require.False(t, isExist, "Init secret should be deleted after first run")

		// Second run: verify deletion is idempotent
		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.NoError(t, err)

		isExist, _, err = getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)

		require.False(t, isExist, "Init secret should remain deleted")
	})

	t.Run("not legacy - should delete init secret once applied", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder()

		// Setup initial state with a non-applied init secret
		err := createInitSecret(ctx, kubeClient, false)
		require.NoError(t, err)

		isExist, isApplied, err := getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)

		require.True(t, isExist, "Init secret should exist initially")
		require.False(t, isApplied, "Init secret should not be applied initially")

		// First run: preserve secret while not yet applied by the PKI hook.
		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())

		isExist, _, err = getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)

		require.True(t, isExist, "Init secret should be preserved while not applied")

		// Second run: once the PKI hook marks the secret applied, the new-arch
		// readiness no longer consults registry-state — applied init alone is
		// sufficient to remove the secret.
		err = createInitSecret(ctx, kubeClient, true)
		require.NoError(t, err)

		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.NoError(t, err)

		isExist, _, err = getInitSecretStatus(ctx, kubeClient)
		require.NoError(t, err)
		require.False(t, isExist, "Init secret should be deleted once applied")
	})
}
