/*
Copyright 2021 Flant CJSC

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
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	challengesSnapshot = "challenges"
	secretsSnapshot    = "registry_secrets_namespaces"

	solverSecretName = "acme-solver-deckhouse-regestry"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cert-manager/registry-secrets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       challengesSnapshot,
			ApiVersion: "certmanager.k8s.io/v1alpha1",
			Kind:       "Challenge",
			FilterFunc: filterNamespace,
		},
		{
			Name:       secretsSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: filterNamespace,
			NameSelector: &types.NameSelector{
				MatchNames: []string{solverSecretName},
			},
		},
	},
}, handleChallenge)

func filterNamespace(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetNamespace(), nil
}

func namespacesFromSnapshot(name string, input *go_hook.HookInput) map[string]struct{} {
	res := map[string]struct{}{}

	snap := input.Snapshots[name]
	for _, n := range snap {
		ns := n.(string)
		res[ns] = struct{}{}
	}

	return res
}

func prepareSolverRegistrySecret(namespace string, dockerCfg string) *v1.Secret {
	return &v1.Secret{
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

		StringData: map[string]string{
			".dockerconfigjson": dockerCfg,
		},

		Type: v1.SecretTypeDockerConfigJson,
	}
}

// handleChallenge
// synopsis:
//   For every namespace contained cert-manager challenge
//   must be contains registry secret for pull challenge solver
//   Because solver image re-pushed in deckhouse registry
//   It need for "closed loop" infrastructure
//   If in namespace delete all challenges then registry secret must be deleted
//   We do not use ownerReferences because cer-manager may to create
//   multiple challenges in one namespace in one time
//   We may solve this in next way: We patch cert-manager for generating
//   image pull-secrets name dynamically (ex PREFIX+challenge_resource_name)
//   and generate one pullSecret per challenge
//   But, cert manager has pr with adding pullImageSecrets through pod template
//   and this solution not generate pullSecrets name dynamically
//   In future we want to rid all of patches in cert-manager
//   and use vanilla cert-manager
func handleChallenge(input *go_hook.HookInput) error {
	registryCfgRaw := input.Values.Get("global.modulesImages.registryDockercfg").String()
	if registryCfgRaw == "" {
		return fmt.Errorf("registry config is empty")
	}
	// we need to decode from base64 str because 'SecretTypeDockerConfigJson'
	// validate passed data as json config but global.modulesImages.registryDockercfg
	// stored in base64
	registryCfgBytes, err := base64.StdEncoding.DecodeString(registryCfgRaw)
	if err != nil {
		return fmt.Errorf("registry config cannot decoded from base64")
	}
	registryCfg := string(registryCfgBytes)

	challengesNss := namespacesFromSnapshot(challengesSnapshot, input)
	secretsNss := namespacesFromSnapshot(secretsSnapshot, input)

	// create secrets
	for ns := range challengesNss {
		// secret already exists in namespace. do not create or patch
		if _, ok := secretsNss[ns]; ok {
			continue
		}

		secret := prepareSolverRegistrySecret(ns, registryCfg)
		un, err := sdk.ToUnstructured(secret)
		if err != nil {
			return err
		}

		err = input.ObjectPatcher.CreateOrUpdateObject(un, "")
		if err != nil {
			return err
		}
	}

	// gc secrets
	for ns := range secretsNss {
		if _, ok := challengesNss[ns]; ok {
			// a secret exists in namespace, and exists one more challenges. do not delete secret
			continue
		}

		err := input.ObjectPatcher.DeleteObject("v1", "Secret", ns, solverSecretName, "")
		if err != nil {
			return err
		}
	}

	return nil
}
