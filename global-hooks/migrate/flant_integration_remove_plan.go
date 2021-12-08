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

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(flantIntegrationPlanRemovalMigration))

func flantIntegrationPlanRemovalMigration(input *go_hook.HookInput, dc dependency.Container) error {
	const cmKey = "flantIntegration"

	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	cm, err := kubeCl.CoreV1().
		ConfigMaps("d8-system").
		Get(context.TODO(), "deckhouse", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get ConfigMap/d8-system/deckhouse: %v", err)
	}

	configYaml, ok := cm.Data[cmKey]
	if !ok {
		input.LogEntry.Warnf("key %q not found in ConfigMap/d8-system/deckhouse", cmKey)
		return nil
	}
	input.LogEntry.Warn(configYaml) // Backup through logs

	config := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(configYaml), &config); err != nil {
		return fmt.Errorf("cannot unmarshal flant-integration config: %v", err)
	}

	delete(config, "plan") // The action

	newConfigYaml, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("cannot marshal flant-integration config: %v", err)
	}

	if len(newConfigYaml) == 0 {
		input.LogEntry.Warnf("new %q value is empty", cmKey)
		return nil
	}

	cm.Data[cmKey] = string(newConfigYaml)

	// Do not retry on conflict, fail and start the hook one more time instead
	_, err = kubeCl.CoreV1().
		ConfigMaps("d8-system").
		Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cannot update ConfigMap/d8-system/deckhouse: %v", err)
	}
	return nil
}
