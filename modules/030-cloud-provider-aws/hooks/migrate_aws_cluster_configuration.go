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

// TODO remove after 1.38 release

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const DataKey = "cloud-provider-cluster-configuration.yaml"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(providerClusterConfigurationMigration))

func providerClusterConfigurationMigration(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	secret, err := kubeCl.CoreV1().
		Secrets("kube-system").
		Get(context.TODO(), "d8-provider-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get Secret/kube-system/d8-provider-cluster-configuration: %v", err)
	}

	configYaml, ok := secret.Data[DataKey]
	if !ok {
		input.LogEntry.Warnf("key %q not found in Secret/kube-system/d8-provider-cluster-configuration", DataKey)
		return nil
	}

	input.LogEntry.Warn(configYaml) // Backup through logs

	config := make(map[string]interface{})
	if err := yaml.Unmarshal(configYaml, &config); err != nil {
		return fmt.Errorf("cannot unmarshal %s config: %v", DataKey, err)
	}

	if _, ok := config["standard"]; !ok {
		input.LogEntry.Info("parameters for layout Standard not found, migration is not needed")
		return nil
	}

	delete(config, "standard")

	configYaml, err = yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("cannot marshal %s config: %v", DataKey, err)
	}

	secret.Data[DataKey] = configYaml

	_, err = kubeCl.CoreV1().
		Secrets("kube-system").
		Update(context.TODO(), secret, metav1.UpdateOptions{})

	return err
}
