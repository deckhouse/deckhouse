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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	defaultLegacyDeschedulerConfigJSON = `{
"removePodsViolatingInterPodAntiAffinity": true,
"removePodsViolatingNodeAffinity": true
}
`

	migrationCM = "descheduler-config-migration"
	migrationNS = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(deschedulerConfigMigration))

func deschedulerConfigMigration(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	mcGVR := schema.ParseGroupResource("moduleconfigs.deckhouse.io").WithVersion("v1alpha1")

	_, err = kubeCl.CoreV1().ConfigMaps(migrationNS).Get(context.TODO(), migrationCM, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get: %w", err)
	}
	if !errors.IsNotFound(err) {
		input.Logger.Info("Migration cm already exists, skipping migration", slog.String("name", migrationCM))
		return nil
	}

	moduleConfig, err := kubeCl.Dynamic().Resource(mcGVR).Get(context.TODO(), "descheduler", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		input.Logger.Info("ModuleConfig for descheduler does not exists, migrating with default config")
	} else if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	deschedulerSettings, exists, err := extractDeschedulerSettingsFromMC(moduleConfig)
	if err != nil {
		return err
	}

	var deschedulerConfigJSON []byte
	if exists && len(deschedulerSettings) > 0 {
		deschedulerConfigJSON, err = json.Marshal(deschedulerSettings)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
	} else {
		deschedulerConfigJSON = []byte(defaultLegacyDeschedulerConfigJSON)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      migrationCM,
			Namespace: migrationNS,
		},
		Data: map[string]string{"config": string(deschedulerConfigJSON)},
	}

	input.PatchCollector.CreateOrUpdate(cm)

	return nil
}

func extractDeschedulerSettingsFromMC(mc *unstructured.Unstructured) (map[string]interface{}, bool, error) {
	if mc == nil {
		return nil, false, nil
	}

	moduleEnabled, exists, err := unstructured.NestedBool(mc.UnstructuredContent(), "spec", "enabled")
	if err != nil {
		return nil, false, fmt.Errorf("nested bool: %w", err)
	}
	if exists && !moduleEnabled {
		return nil, false, nil
	}

	deschedulerSettings, _, err := unstructured.NestedMap(mc.UnstructuredContent(), "spec", "settings")
	if err != nil {
		return deschedulerSettings, false, fmt.Errorf("nested map: %w", err)
	}

	return deschedulerSettings, true, nil
}
