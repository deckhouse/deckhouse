/*
Copyright 2023 Flant JSC

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
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

/* Cleanup:
This hook removes the orphan kube-system/d8-cni-configuration secret if there is at least one cni moduleConfig.
If secret doesn't exist, cleanup skipped.
If module config for cni doesn't exist, cleanup skipped.
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 20},
}, dependency.WithExternalDependencies(d8cniSecretCleanup))

func d8cniSecretCleanup(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	// skip cleanup if d8-cni-configuration secret doesn't exist.
	d8cniSecret, err := kubeCl.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cni-configuration", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		input.Logger.Info("d8-cni-configuration secret does not exist, skipping cleanup")
		return nil
	}
	if err != nil {
		return err
	}

	moduleConfigs, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	for _, mc := range moduleConfigs.Items {
		if slices.Contains([]string{"cni-cilium", "cni-flannel", "cni-simple-bridge"}, mc.GetName()) {
			moduleEnabled, exists, err := unstructured.NestedBool(mc.UnstructuredContent(), "spec", "enabled")
			if err != nil {
				return err
			}
			if !exists {
				break
			}
			if !moduleEnabled {
				break
			}

			input.Logger.Infof("Module config for %s found, secret will be cleaned", mc.GetName())
			return removeD8CniSecret(input, kubeCl, d8cniSecret)
		}
	}
	input.Logger.Infof("No one enabled moduleConfig of CNI is found, skipping cleanup")
	return nil
}

// remove secret
func removeD8CniSecret(input *go_hook.HookInput, kubeCl k8s.Client, secret *v1.Secret) error {
	var secretData []byte
	err := secret.Unmarshal(secretData)
	if err != nil {
		return err
	}
	input.Logger.Info(string(secretData))
	return kubeCl.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
}
