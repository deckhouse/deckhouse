/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	challengesSnapshot = "challenges"
	secretsSnapshot    = "registry_secrets_namespaces"
	saSnapshot         = "sa_namespaces"
	d8RegistrySnapshot = "d8_registry_secret"

	solverSecretName         = "acme-solver-deckhouse-regestry"
	solverServiceAccountName = "acme-solver-deckhouse-sa"
)

type registrySecret struct {
	Namespace string
	Config    string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cert-manager/registry-secrets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       challengesSnapshot,
			ApiVersion: "acme.cert-manager.io/v1",
			Kind:       "Challenge",
			FilterFunc: applyNamespaceFilter,
		},
		{
			Name:       secretsSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyRegistrySecretFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{solverSecretName},
			},
		},
		{
			Name:       saSnapshot,
			Kind:       "ServiceAccount",
			ApiVersion: "v1",
			FilterFunc: applyServiceAccountFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{solverServiceAccountName},
			},
		},
		{
			Name:       d8RegistrySnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyRegistrySecretFilter,
		},
	},
}, handleChallenge)

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetNamespace(), nil
}

func applyRegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s corev1.Secret
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	ns := s.GetNamespace()

	conf, ok := s.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return "", fmt.Errorf("registry auth conf is not in registry secret %s/%s", ns, obj.GetName())
	}

	return registrySecret{
		Namespace: ns,
		Config:    string(conf),
	}, nil
}

func applyServiceAccountFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s corev1.ServiceAccount
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	return s.GetNamespace(), nil
}

func prepareSolverRegistrySecret(namespace, dockerCfg string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      solverSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"cert-manager.deckhouse.io/solver-registry-secret": "true",
				"deckhouse.io/registry-secret":                     "true",
			},
		},

		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(dockerCfg),
		},

		Type: corev1.SecretTypeDockerConfigJson,
	}
}

func prepareSolverRegistryServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      solverServiceAccountName,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage":                            "deckhouse",
				"cert-manager.deckhouse.io/solver-sa": "true",
			},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: solverSecretName}},
	}
}

// handleChallenge
// synopsis:
//
//	For every namespace contained cert-manager challenge
//	must be contains registry secret for pull challenge solver
//	Because solver image re-pushed in deckhouse registry
//	It need for "closed loop" infrastructure
//	If in namespace delete all challenges then registry secret must be deleted
//	We do not use ownerReferences because cer-manager may to create
//	multiple challenges in one namespace in one time
//	We may solve this in next way: We patch cert-manager for generating
//	image pull-secrets name dynamically (ex PREFIX+challenge_resource_name)
//	and generate one pullSecret per challenge
//	But, cert manager has pr with adding pullImageSecrets through pod template
//	and this solution not generate pullSecrets name dynamically
//	In future we want to rid all of patches in cert-manager
//	and use vanilla cert-manager
func handleChallenge(_ context.Context, input *go_hook.HookInput) error {
	d8RegistrySnap := input.Snapshots.Get(d8RegistrySnapshot)
	if len(d8RegistrySnap) == 0 {
		input.Logger.Warn("Registry secret not found. Skip")
		return nil
	}

	var regSecret registrySecret
	err := d8RegistrySnap[0].UnmarshalTo(&regSecret)
	if err != nil {
		return fmt.Errorf("failed to unmarshal d8_registry_secret snapshot: %w", err)
	}

	registryCfg := regSecret.Config

	challengesNss := set.NewFromSnapshot(input.Snapshots.Get(challengesSnapshot))

	serviceAccountsNss := set.NewFromSnapshot(input.Snapshots.Get(saSnapshot))

	// namespace -> .dockerconfigjson content
	secretsByNs := map[string]string{}

	for regSecret, err := range sdkobjectpatch.SnapshotIter[registrySecret](input.Snapshots.Get(secretsSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'd8_registry_secret' snapshot: %w", err)
		}

		secretsByNs[regSecret.Namespace] = regSecret.Config
	}

	// create secrets
	for ns := range challengesNss {
		secretContent, secretExists := secretsByNs[ns]
		// secret already exists in namespace. do not create or patch
		if !secretExists || secretContent != registryCfg {
			secret := prepareSolverRegistrySecret(ns, registryCfg)
			input.PatchCollector.CreateOrUpdate(secret)
		}

		if _, saExists := serviceAccountsNss[ns]; !saExists {
			sa := prepareSolverRegistryServiceAccount(ns)
			input.PatchCollector.CreateOrUpdate(sa)
		}
	}

	// gc secrets
	for ns := range secretsByNs {
		if challengesNss.Has(ns) {
			// a secret exists in namespace, and exists one more challenges. do not delete secret
			continue
		}

		// NOTE: This deletion is async because synchronous deletion might hang.
		// Kubernetes GC will handle the actual cleanup.
		input.PatchCollector.DeleteInBackground("v1", "Secret", ns, solverSecretName)
	}

	// gc SA's
	for ns := range serviceAccountsNss {
		if challengesNss.Has(ns) {
			// a service account exists in namespace and one more challenge exists, do not delete secret
			continue
		}

		// NOTE: This deletion is async because synchronous deletion might hang.
		// Kubernetes GC will handle the actual cleanup.
		input.PatchCollector.DeleteInBackground("v1", "ServiceAccount", ns, solverServiceAccountName)
	}

	return nil
}
