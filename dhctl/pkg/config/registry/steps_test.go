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

func TestIsRegistryReady(t *testing.T) {
	t.Run("legacy - returns ready regardless of state secret", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder(
			WithLegacyMode(),
		)

		// Legacy mode skips readiness checks entirely, so the registry is
		// considered ready no matter what the state secret contains.
		err := isRegistryReady(ctx, kubeClient, config)
		require.NoError(t, err)

		err = createStatusSecret(ctx, kubeClient, false)
		require.NoError(t, err)

		err = isRegistryReady(ctx, kubeClient, config)
		require.NoError(t, err)

		err = createStatusSecret(ctx, kubeClient, true)
		require.NoError(t, err)

		err = isRegistryReady(ctx, kubeClient, config)
		require.NoError(t, err)
	})

	// The state secret still decides, on the two modes whose entire content is a registry the
	// previous implementation runs in the cluster.
	t.Run("the previous implementation's own modes - readiness flow", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := client.NewFakeKubernetesClient()
		config := ConfigBuilder(WithModeProxy())

		// First run: not ready when module status is unknown
		err := isRegistryReady(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())

		// Second run: not ready with unready status
		err = createStatusSecret(ctx, kubeClient, false)
		require.NoError(t, err)

		err = isRegistryReady(ctx, kubeClient, config)
		require.EqualError(t, err, ErrIsNotReady.Error())

		// Third run: ready when status becomes ready
		err = createStatusSecret(ctx, kubeClient, true)
		require.NoError(t, err)

		err = isRegistryReady(ctx, kubeClient, config)
		require.NoError(t, err)
	})
}

// TestNothingWaitsOnAStateSecretThatIsNeverWritten is the wait that cost a working cluster its
// installation.
//
// `registry-state` is written from values the current implementation clears the moment it owns the
// cluster, which is every cluster installed from now on. Yet Direct — the default for a cluster whose
// container runtime supports it, and what a plain `mode: Managed` with an upstream resolves to — used
// to be read as "not legacy, so the previous implementation reports on this" and waited on that
// secret. A hundred attempts, twenty seconds apart, and then a failed installation of a cluster that
// was already pulling images.
//
// The fake client below has no state secret and no RegistryStorage in it, which is exactly the point:
// on these configurations nothing must be asked for at all.
func TestNothingWaitsOnAStateSecretThatIsNeverWritten(t *testing.T) {
	for _, mode := range []struct {
		name   string
		config Config
	}{
		{"direct, the default for a supported runtime", ConfigBuilder(WithModeDirect())},
		{"unmanaged", ConfigBuilder(WithModeUnmanaged())},
		{"unmanaged, from a legacy initConfiguration", ConfigBuilder(WithLegacyMode())},
	} {
		t.Run(mode.name, func(t *testing.T) {
			require.NoError(t, isRegistryReady(t.Context(), client.NewFakeKubernetesClient(), mode.config))
		})
	}
}

// TestAStoreIsWaitedForOnlyWhereItIsTheOnlySource pins which installations may proceed with a cache
// that is still filling, and which may not.
//
// A cache beside an upstream is an optimisation: the agent falls back to the upstream for anything not
// copied yet, so the cluster pulls everything from the moment it exists. Waiting for the cache anyway
// means waiting for the whole first sync, because the store reports `Ready` only once its leader is
// FULL — measured on a three-master cache cluster as twelve gigabytes and about fifteen minutes of a
// silent log, ended by the bootstrap watchdog killing the installation.
//
// Without an upstream the cache is the only source there is, and an installation that proceeds past a
// half-filled one hands the cluster nodes that cannot pull.
func TestAStoreIsWaitedForOnlyWhereItIsTheOnlySource(t *testing.T) {
	t.Run("a cache beside an upstream is not waited for", func(t *testing.T) {
		config := ConfigBuilder(WithModeDirect())
		config.StoreExpected = true

		require.NoError(t, isRegistryReady(t.Context(), client.NewFakeKubernetesClient(), config),
			"the cluster pulls through its upstream, so a filling cache must not hold the installation")
	})

	t.Run("a bundle installation waits, and an absent store is worth retrying", func(t *testing.T) {
		config := ConfigBuilder(WithModeLocal())
		config.StoreExpected = true
		config.BundleBootstrap = true

		err := isRegistryReady(t.Context(), client.NewFakeKubernetesClient(), config)
		require.Error(t, err, "an absent RegistryStorage is not a store that is ready")
		require.ErrorIs(t, err, errRegistryCheckTransient,
			"the object appears when the module starts, which can be after this point, so this has to "+
				"be worth retrying rather than fatal")
	})
}
