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

package deckhouse_config

import (
	"context"
	"fmt"
	"testing"

	"github.com/flant/kube-client/client"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
)

func TestInitialConfigLoaderWithConfigMapDeckhouse(t *testing.T) {
	// Define some conversions with deletions.
	conversion.RegisterFunc("global", 1, 2, func(settings *conversion.Settings) error {
		return settings.Delete("paramGroup.obsoleteParam")
	})
	conversion.RegisterFunc("module-one", 1, 2, func(settings *conversion.Settings) error {
		return settings.Delete("paramGroup.obsoleteParam")
	})

	// KubeClient
	kubeClient := client.NewFake(nil)
	loader := NewInitialConfigLoader(kubeClient)

	t.Run("No configMaps", func(t *testing.T) {
		g := NewWithT(t)

		cfg, err := loader.GetInitialKubeConfig(GeneratedConfigMapName)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(cfg).Should(BeNil())

		cfg, err = loader.GetInitialKubeConfig(GeneratedConfigMapName)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(cfg).Should(BeNil())
	})

	t.Run("cm/deckhouse", func(t *testing.T) {
		g := NewWithT(t)
		err := createCm(kubeClient, fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  global: |
    paramGroup:
      param1: value1
      obsoleteParam: value2
  moduleOne: |
    paramGroup:
      param1: value1
      obsoleteParam: value2
`, DeckhouseConfigMapName, DeckhouseNS))
		g.Expect(err).ShouldNot(HaveOccurred(), "should create cm in fake client")

		kubeCfg, err := loader.GetInitialKubeConfig(DeckhouseConfigMapName)
		g.Expect(err).ShouldNot(HaveOccurred(), "should load KubeConfig from cm")
		g.Expect(kubeCfg).ShouldNot(BeNil(), "KubeConfig should not be nil")

		g.Expect(kubeCfg.Global).ShouldNot(BeNil(), "should have global")
		g.Expect(kubeCfg.Global.Values).ShouldNot(BeNil(), "should have global values")

		ms, err := conversion.SettingsFromMap(kubeCfg.Global.Values)
		g.Expect(err).ShouldNot(HaveOccurred(), "should wrap global values")
		g.Expect(ms.Get("global.paramGroup.param1").Exists()).Should(BeTrue(), "should have param1, got %+v", ms.String())
		g.Expect(ms.Get("global.paramGroup.obsoleteParam").Exists()).ShouldNot(BeTrue(), "should not have obsolete param, got %+v", ms.String())

		g.Expect(kubeCfg.Modules).Should(HaveKey("module-one"), "should have module config")
		ms, err = conversion.SettingsFromMap(kubeCfg.Modules["module-one"].Values)
		g.Expect(err).ShouldNot(HaveOccurred(), "should wrap 'module-one' values")
		g.Expect(ms.Get("moduleOne.paramGroup.param1").Exists()).Should(BeTrue(), "should have param1, got %+v", ms.String())
		g.Expect(ms.Get("moduleOne.paramGroup.obsoleteParam").Exists()).ShouldNot(BeTrue(), "should not have obsolete param, got %+v", ms.String())
	})

	t.Run("ModuleConfigs", func(t *testing.T) {
		g := NewWithT(t)
		err := createCm(kubeClient, fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  global: |
    paramGroup:
      param1: value1
      obsoleteParam: value2
  moduleOne: |
    paramGroup:
      param1: value1
      obsoleteParam: value2
  moduleThreeEnabled: "true"
  moduleFourEnabled: "true"
`, GeneratedConfigMapName, DeckhouseNS))
		g.Expect(err).ShouldNot(HaveOccurred(), "should create cm/%s in fake client", GeneratedConfigMapName)

		err = createModCfg(kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramGroup:
      param1: value1
      obsoleteParam: value2
`)
		g.Expect(err).ShouldNot(HaveOccurred(), "should create ModuleConfig/global in fake client")

		err = createModCfg(kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  version: 1
  settings:
    paramGroup:
      param1: value1
      obsoleteParam: value2
`)
		g.Expect(err).ShouldNot(HaveOccurred(), "should create ModuleConfig/module-one in fake client")

		// Extra module — not in generated ConfigMap: unknown or new.
		err = createModCfg(kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-two
spec:
  version: 1
  settings:
    param1: value1
    param2: value2
  enabled: false
`)
		g.Expect(err).ShouldNot(HaveOccurred(), "should create ModuleConfig/module-two in fake client")

		// Enabled only module.
		err = createModCfg(kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-three
spec:
  enabled: false
`)
		g.Expect(err).ShouldNot(HaveOccurred(), "should create ModuleConfig/module-three in fake client")

		cfgs, err := GetAllConfigs(kubeClient)
		g.Expect(err).ShouldNot(HaveOccurred(), "should list ModuleConfigs")
		g.Expect(cfgs).Should(HaveLen(4))

		kubeCfg, err := loader.GetInitialKubeConfig(GeneratedConfigMapName)
		g.Expect(err).ShouldNot(HaveOccurred(), "should load KubeConfig from cm")
		g.Expect(kubeCfg).ShouldNot(BeNil(), "KubeConfig should not be nil")

		g.Expect(kubeCfg.Global).ShouldNot(BeNil(), "should have global")
		g.Expect(kubeCfg.Global.Values).ShouldNot(BeNil(), "should have global values")

		ms, err := conversion.SettingsFromMap(kubeCfg.Global.Values)
		g.Expect(err).ShouldNot(HaveOccurred(), "should wrap global values")
		g.Expect(ms.Get("global.paramGroup.param1").Exists()).Should(BeTrue(), "should have param1, got %+v", ms.String())
		g.Expect(ms.Get("global.paramGroup.obsoleteParam").Exists()).ShouldNot(BeTrue(), "should not have obsolete param, got %+v", ms.String())

		g.Expect(kubeCfg.Modules).Should(HaveKey("module-one"), "should have module config")
		ms, err = conversion.SettingsFromMap(kubeCfg.Modules["module-one"].Values)
		g.Expect(err).ShouldNot(HaveOccurred(), "should wrap 'module-one' values")
		g.Expect(ms.Get("moduleOne.paramGroup.param1").Exists()).Should(BeTrue(), "should have param1, got %+v", ms.String())
		g.Expect(ms.Get("moduleOne.paramGroup.obsoleteParam").Exists()).ShouldNot(BeTrue(), "should not have obsolete param, got %+v", ms.String())

		g.Expect(kubeCfg.Modules).ShouldNot(HaveKey("module-two"), "should not have extra module config")

		g.Expect(kubeCfg.Modules).Should(HaveKey("module-three"), "should have module config module-three")
		g.Expect(kubeCfg.Modules["module-three"].Values).Should(HaveLen(0), "module-three should not have values")
		g.Expect(kubeCfg.Modules["module-three"].IsEnabled).ShouldNot(BeNil(), "module-three should have enabled")
		g.Expect(*kubeCfg.Modules["module-three"].IsEnabled).Should(BeFalse(), "module-three should have enabled from ModuleConfig")

		g.Expect(kubeCfg.Modules).ShouldNot(HaveKey("module-four"), "should drop configs if no ModuleConfig present")
	})
}

func createCm(kubeClient client.Client, manifest string) error {
	var cm v1.ConfigMap
	err := yaml.Unmarshal([]byte(manifest), &cm)
	if err != nil {
		return err
	}

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cm)
	if err != nil {
		return err
	}
	obj := &unstructured.Unstructured{Object: content}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	_, err = kubeClient.Dynamic().Resource(gvr).Namespace(DeckhouseNS).Create(context.Background(), obj, metav1.CreateOptions{})
	return err
}

func createModCfg(kubeClient client.Client, manifest string) error {
	var cfg d8cfg_v1alpha1.ModuleConfig
	err := yaml.Unmarshal([]byte(manifest), &cfg)
	if err != nil {
		return err
	}

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cfg)
	if err != nil {
		return err
	}
	obj := &unstructured.Unstructured{Object: content}

	_, err = kubeClient.Dynamic().Resource(d8cfg_v1alpha1.GroupVersionResource()).Create(context.Background(), obj, metav1.CreateOptions{})
	return err
}
