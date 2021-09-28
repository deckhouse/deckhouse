// Copyright 2021 Flant JSC
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

package hooks

// TODO remove this hook on next release !!!

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

type secretInfo struct {
	Name      string
	Namespace string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(addValidationLabelToSecret))

func addValidationLabelToSecret(input *go_hook.HookInput, dc dependency.Container) error {
	patchedSecrets, err := getSecretsList(dc)
	if err != nil {
		return err
	}

	for _, secret := range patchedSecrets {
		input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var s v1core.Secret
			err := sdk.FromUnstructured(obj, &s)
			if err != nil {
				return nil, err
			}
			s.Labels["name"] = s.Name
			return sdk.ToUnstructured(&s)
		}, "v1", "Secret", secret.Namespace, secret.Name)
	}

	return nil
}

func getSecretsList(dc dependency.Container) ([]secretInfo, error) {
	expectedSecrets := []secretInfo{
		{"d8-cluster-terraform-state", "d8-system"},
		{"d8-cluster-configuration", "kube-system"},
		{"d8-provider-cluster-configuration", "kube-system"},
		{"d8-static-cluster-configuration", "kube-system"},
		{"d8-masters-kubernetes-data-device-path", "d8-system"},
	}

	result := make([]secretInfo, 0, len(expectedSecrets))

	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	for _, s := range expectedSecrets {
		_, err := kubeCl.CoreV1().
			Secrets(s.Namespace).
			Get(context.TODO(), s.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		result = append(result, s)
	}

	// Get secrets d8-node-terraform-state-*
	secretList, err := kubeCl.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{LabelSelector: "node.deckhouse.io/terraform-state="})
	if err != nil {
		return nil, fmt.Errorf("cannot get secretList for d8-node-terraform-state-* secrets")
	}

	for _, s := range secretList.Items {
		result = append(result, secretInfo{Name: s.Name, Namespace: s.Namespace})
	}

	return result, nil
}
