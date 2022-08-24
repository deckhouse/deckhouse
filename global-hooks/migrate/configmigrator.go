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
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

func newModuleConfigMigrator(dc dependency.Container, input *go_hook.HookInput) (*moduleConfigMigrator, error) {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	configMigrator := &moduleConfigMigrator{
		logger: input.LogEntry,
		klient: kubeCl,
	}

	return configMigrator, nil
}

type moduleConfigMigrator struct {
	cm     *v1.ConfigMap
	logger *logrus.Entry
	klient k8s.Client
}

func (m *moduleConfigMigrator) getConfig(cmKey string) (map[string]interface{}, error) {
	cm, err := m.klient.CoreV1().
		ConfigMaps("d8-system").
		Get(context.TODO(), "deckhouse", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot get ConfigMap/d8-system/deckhouse: %v", err)
	}

	m.cm = cm // store for update

	configYaml, ok := cm.Data[cmKey]
	if !ok {
		m.logger.Warnf("key %q not found in ConfigMap/d8-system/deckhouse", cmKey)
		return nil, nil
	}
	m.logger.Warn(configYaml) // Backup through logs

	config := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(configYaml), &config); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s config: %v", cmKey, err)
	}

	return config, nil
}

func (m *moduleConfigMigrator) setConfig(cmKey string, config map[string]interface{}) error {
	if len(config) == 0 {
		delete(m.cm.Data, cmKey)
	} else {
		newConfigYaml, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("cannot marshal %s config: %v", cmKey, err)
		}
		m.cm.Data[cmKey] = string(newConfigYaml)
	}

	// Do not retry on conflict, fail and start the hook one more time instead
	_, err := m.klient.CoreV1().
		ConfigMaps("d8-system").
		Update(context.TODO(), m.cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cannot update ConfigMap/d8-system/deckhouse: %v", err)
	}
	return nil
}
