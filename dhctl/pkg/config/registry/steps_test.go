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

func createInitSecret(ctx context.Context, kubeClient client.KubeClient) error {
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

	return createOrUpdateSecret(ctx, kubeClient, secret)
}

func createStatusSecret(ctx context.Context, kubeClient client.KubeClient, ready bool) error {
	conditions := make([]metav1.Condition, 0)

	if ready {
		conditions = append(conditions, metav1.Condition{
			Type:   conditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	}

	conditionsYaml, err := yaml.Marshal(conditions)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateSecretName,
			Namespace: secretsNamespace,
		},
		Data: map[string][]byte{
			"conditions": conditionsYaml,
		},
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

func TestCheckRegistryInitialization(t *testing.T) {
	t.Run("legacy - returns ready regardless of state secret", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder(
			WithLegacyMode(),
		)

		// Legacy mode skips readiness checks entirely.
		err := checkRegistryInitialization(ctx, kubeClient, config)
		require.NoError(t, err)
	})

	t.Run("not legacy - not ready when state secret is missing", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder()

		// No state secret means conditions are unknown, so the registry is not ready.
		err := checkRegistryInitialization(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())
	})

	t.Run("not legacy - readiness flow", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder()

		// First run: not ready when module status is unknown
		err := checkRegistryInitialization(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())

		// Second run: not ready with unready status
		err = createStatusSecret(ctx, kubeClient, false)
		require.NoError(t, err)

		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())

		// Third run: ready when status becomes ready
		err = createStatusSecret(ctx, kubeClient, true)
		require.NoError(t, err)

		err = checkRegistryInitialization(ctx, kubeClient, config)
		require.NoError(t, err)
	})
}
