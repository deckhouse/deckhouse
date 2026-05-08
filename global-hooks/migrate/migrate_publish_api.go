/*
Copyright 2026 Flant JSC

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
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	temporaryConfigCM = "d8-publishapi-config-migration"
	targetNS          = "kube-system"
)

type ModuleConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleConfigSpec   `json:"spec"`
	Status ModuleConfigStatus `json:"status,omitempty"`
}

type ModuleConfigSpec struct {
	Version  int            `json:"version,omitempty"`
	Settings SettingsValues `json:"settings,omitempty"`
	Enabled  bool           `json:"enabled,omitempty"`
}

type SettingsValues struct {
	PublishAPI *struct{} `json:"publishAPI" yaml:"publishAPI"`
}

type ModuleConfigStatus struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

func applyModuleConfigFilterForMigration(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert user-authn ModuleConfig: %v", err)
	}
	return mc.Spec.Settings.PublishAPI, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_config_authn",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"user-authn"},
			},
			FilterFunc: applyModuleConfigFilterForMigration,
		},
	},
}, dependency.WithExternalDependencies(publishAPIConfigMigration))

func publishAPIConfigMigration(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	mcGVR := schema.ParseGroupResource("moduleconfigs.deckhouse.io").WithVersion("v1alpha1")

	moduleConfig, err := kubeCl.Dynamic().Resource(mcGVR).Get(context.TODO(), "user-authn", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		input.Logger.Info("ModuleConfig for user-authn does not exists, skipping PublishAPI migration")
		err := cleanupMigrationCm(input, kubeCl)
		if err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	publishAPISettings, exists, err := extractPublishAPISettingsFromMC(moduleConfig)
	if err != nil {
		return err
	}

	var publishAPIConfigJSON []byte
	if exists && len(publishAPISettings) > 0 {
		publishAPIConfigJSON, err = json.Marshal(publishAPISettings)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
	} else {
		input.Logger.Info("Looks like publish API ingress settings are not set in ModuleConfig user-authn, skipping")
		err := cleanupMigrationCm(input, kubeCl)
		if err != nil {
			return err
		}
		return nil
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      temporaryConfigCM,
			Namespace: targetNS,
		},
		Data: map[string]string{"config": string(publishAPIConfigJSON)},
	}

	input.PatchCollector.CreateOrUpdate(cm)
	input.Logger.Info("Written exported publish API ingress settings from ModuleConfig user-authn to temporary ConfigMap", slog.String("configmap", temporaryConfigCM), slog.String("namespace", targetNS))

	return nil
}

func extractPublishAPISettingsFromMC(mc *unstructured.Unstructured) (map[string]interface{}, bool, error) {
	if mc == nil {
		return nil, false, nil
	}

	publishAPISettings, exists, err := unstructured.NestedMap(mc.UnstructuredContent(), "spec", "settings", "publishAPI")
	if err != nil {
		return publishAPISettings, false, fmt.Errorf("nested map: %w", err)
	} else if !exists {
		return nil, false, nil
	}

	return publishAPISettings, true, nil
}

func cleanupMigrationCm(input *go_hook.HookInput, kubeCl k8s.Client) error {
	_, err := kubeCl.CoreV1().ConfigMaps(targetNS).Get(context.TODO(), temporaryConfigCM, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get: %w", err)
	}
	if !errors.IsNotFound(err) {
		input.Logger.Info("Migration configmap exists while user-authn publishAPI settings are empty, cleaning up the configmap", slog.String("name", temporaryConfigCM))
		err = kubeCl.CoreV1().ConfigMaps(targetNS).Delete(context.TODO(), temporaryConfigCM, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete: %w", err)
		}
	}
	return nil
}
