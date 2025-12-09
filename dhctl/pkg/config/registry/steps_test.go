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

	"github.com/stretchr/testify/assert"
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
	rawYaml, err := yaml.Marshal(pki)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      initSecretName,
			Namespace: secretsNamespace,
		},
		Data: map[string][]byte{
			"config": rawYaml,
		},
	}
	if applied {
		secret.Annotations = map[string]string{
			initSecretAppliedAnnotation: "",
		}
	}
	return createOrUpdateSecret(ctx, kubeClient, secret)
}

func createStatusSecret(ctx context.Context, kubeClient client.KubeClient, ready bool) error {
	conditions := make([]metav1.Condition, 0)
	if ready {
		conditions = append(conditions, metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		})
	}
	rawYaml, err := yaml.Marshal(conditions)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateSecretName,
			Namespace: secretsNamespace,
		},
		Data: map[string][]byte{
			"conditions": rawYaml,
		},
	}
	return createOrUpdateSecret(ctx, kubeClient, secret)
}

func createOrUpdateSecret(ctx context.Context, kubeClient client.KubeClient, secret *corev1.Secret) error {
	_, err := kubeClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		_, err = kubeClient.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

func TestCheckRegistryInitialization(t *testing.T) {
	t.Run("legacy - should delete init secret", func(t *testing.T) {
		config := TestConfigBuilder(WithLegacyMode())
		kubeClient := client.NewFakeKubernetesClient()

		// Setup: create non-applied init secret
		require.NoError(t, createInitSecret(t.Context(), kubeClient, false))

		isExist, isApplied, err := getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.True(t, isExist, "Init secret should exist initially")
		assert.False(t, isApplied, "Init secret should not be applied initially")

		// First run: delete the secret
		err = checkRegistryInitialization(t.Context(), kubeClient, config)
		assert.NoError(t, err)

		isExist, _, err = getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.False(t, isExist, "Init secret should be deleted after first run")

		// Second run: verify deletion is idempotent
		err = checkRegistryInitialization(t.Context(), kubeClient, config)
		assert.NoError(t, err)

		isExist, _, err = getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.False(t, isExist, "Init secret should remain deleted")
	})

	t.Run("not legacy - should delete init secret after ready", func(t *testing.T) {
		config := TestConfigBuilder()
		kubeClient := client.NewFakeKubernetesClient()

		// Setup initial state with applied init secret
		require.NoError(t, createInitSecret(t.Context(), kubeClient, true))

		isExist, isApplied, err := getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.True(t, isExist, "Init secret should exist initially")
		assert.True(t, isApplied, "Init secret should be applied initially")

		// First run: preserve secret when module status is unknown
		err = checkRegistryInitialization(t.Context(), kubeClient, config)
		assert.EqualError(t, err, ErrIsNotReady.Error())

		isExist, _, err = getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.True(t, isExist, "Init secret should be preserved when module status is unknown")

		// Second run: preserve secret with unready status
		require.NoError(t, createStatusSecret(t.Context(), kubeClient, false))

		err = checkRegistryInitialization(t.Context(), kubeClient, config)
		assert.EqualError(t, err, ErrIsNotReady.Error())

		isExist, _, err = getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.True(t, isExist, "Init secret should remain preserved with unready status")

		// Third run: delete secret when status becomes ready
		require.NoError(t, createStatusSecret(t.Context(), kubeClient, true))

		err = checkRegistryInitialization(t.Context(), kubeClient, config)
		assert.NoError(t, err)

		isExist, _, err = getInitSecretStatus(t.Context(), kubeClient)
		require.NoError(t, err)
		assert.False(t, isExist, "Init secret should be deleted when module becomes ready")
	})
}
