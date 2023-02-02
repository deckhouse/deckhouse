/*
Copyright 2022 Flant JSC

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

// TODO remove after 1.42 release

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(deschedulerConfigMigration))

func deschedulerConfigMigration(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	mcGVR := schema.ParseGroupResource("moduleconfigs.deckhouse.io").WithVersion("v1alpha1")

	moduleConfig, err := kubeCl.Dynamic().Resource(mcGVR).Get(context.TODO(), "descheduler", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		input.LogEntry.Info("ModuleConfig for descheduler does not exists, nothing to migrate")
		return nil
	} else if err != nil {
		return err
	}

	deschedulerSettings, exists, err := unstructured.NestedMap(moduleConfig.UnstructuredContent(), "spec", "settings")
	if err != nil {
		return err
	}

	if !exists || len(deschedulerSettings) == 0 {
		input.LogEntry.Info("Config for descheduler is empty, nothing to migrate")
	}

	deschedulerConfigJSON, err := json.Marshal(deschedulerSettings)
	if err != nil {
		return err
	}

	_, err = kubeCl.CoreV1().ConfigMaps("d8-system").Create(context.TODO(), &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "descheduler-config-migration",
			Namespace: "d8-system",
		},
		Data: map[string]string{"config": string(deschedulerConfigJSON)},
	}, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		input.LogEntry.Infof("CM already existis, skipping migration: %s", err)
	} else if err != nil {
		return err
	}

	return nil
}
