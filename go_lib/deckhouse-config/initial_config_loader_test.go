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

	kcm "github.com/flant/addon-operator/pkg/kube_config_manager"
	"github.com/flant/kube-client/client"
	"github.com/flant/kube-client/fake"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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
	mcGVR := d8cfg_v1alpha1.GroupVersionResource()
	fakeCluster := fake.NewFakeCluster(fake.ClusterVersionV121)
	fakeCluster.RegisterCRD(mcGVR.Group, mcGVR.Version, "ModuleConfig", false)
	kubeClient := fakeCluster.Client
	loader := NewInitialConfigLoader(kubeClient)

	t.Run("No configMaps", func(t *testing.T) {
		g := NewWithT(t)

		cfg, err := loader.GetInitialKubeConfig(GeneratedConfigMapName)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(cfg).Should(BeNil())

		cfg, err = loader.GetInitialKubeConfig(DeckhouseConfigMapName)
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

		g.Expect(kubeCfg.Global).Should(HaveField("Values", Not(BeNil())), "should have Global with Values")

		g.Expect(kubeCfg.Global.Values).Should(MatchAllKeys(Keys{
			"global": MatchAllKeys(Keys{
				"paramGroup": MatchAllKeys(Keys{
					"param1": Equal("value1"),
				}),
			}),
		}))

		g.Expect(kubeCfg.Modules).Should(MatchAllKeys(Keys{
			"module-one": PointTo(And(
				HaveField("ModuleConfig.IsEnabled", BeNil()),
				HaveField("ModuleConfig.Values", MatchAllKeys(Keys{
					"moduleOne": MatchAllKeys(Keys{
						"paramGroup": MatchAllKeys(Keys{
							"param1": Equal("value1"),
						}),
					}),
				})),
			)),
		}))
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

		g.Expect(kubeCfg.Global).Should(HaveField("Values", Not(BeNil())), "should have Global with Values")

		g.Expect(kubeCfg.Global.Values).Should(MatchAllKeys(Keys{
			"global": MatchAllKeys(Keys{
				"paramGroup": MatchAllKeys(Keys{
					"param1": Equal("value1"),
				}),
			}),
		}))

		g.Expect(kubeCfg.Modules).Should(MatchAllKeys(Keys{
			"module-one": PointTo(And(
				HaveField("ModuleConfig.IsEnabled", BeNil()),
				HaveField("ModuleConfig.Values", MatchAllKeys(Keys{
					"moduleOne": MatchAllKeys(Keys{
						"paramGroup": MatchAllKeys(Keys{
							"param1": Equal("value1"),
						}),
					}),
				})),
			)),
			"module-three": PointTo(And(
				HaveField("ModuleConfig.IsEnabled", PointTo(BeFalse())),
				HaveField("ModuleConfig.Values", HaveLen(0)),
			)),
		}))
	})
}

func TestInitialConfigLegacyConfigMapToInitialConfig(t *testing.T) {
	// Define some conversions with deletions.
	conversion.RegisterFunc("global", 1, 2, func(settings *conversion.Settings) error {
		return settings.DeleteAndClean("paramGroup.obsoleteParam")
	})
	conversion.RegisterFunc("module-one", 1, 2, func(settings *conversion.Settings) error {
		return settings.DeleteAndClean("paramGroup.obsoleteParam")
	})

	tests := []struct {
		name    string
		data    string
		matcher func(t *testing.T, cfg *kcm.KubeConfig)
	}{
		{
			"global and one module enabled",
			`
global: |
  param1: val1
moduleOneEnabled: "false"
`,
			func(t *testing.T, cfg *kcm.KubeConfig) {
				g := NewWithT(t)

				g.Expect(cfg.Global).Should(HaveField("Values", Not(BeNil())), "should have Global with Values")

				g.Expect(cfg.Global.Values).Should(MatchAllKeys(Keys{
					"global": MatchAllKeys(Keys{
						"param1": Equal("val1"),
					}),
				}))

				g.Expect(cfg.Modules).Should(MatchAllKeys(Keys{
					"module-one": PointTo(And(
						HaveField("ModuleConfig.IsEnabled", PointTo(BeFalse())),
						HaveField("ModuleConfig.Values", HaveLen(0)),
					)),
				}))
			},
		},
		{
			"global and module converted to empty",
			`
global: |
  paramGroup:
    obsoleteParam: someVal
moduleOne: |
  paramGroup:
    obsoleteParam: someVal
`,
			func(t *testing.T, cfg *kcm.KubeConfig) {
				g := NewWithT(t)

				g.Expect(cfg.Global).Should(HaveField("Values", Not(BeNil())), "should have Global with Values")

				g.Expect(cfg.Global.Values).Should(MatchAllKeys(Keys{
					"global": HaveLen(0),
				}))

				g.Expect(cfg.Modules).Should(MatchAllKeys(Keys{
					"module-one": PointTo(And(
						HaveField("ModuleConfig.IsEnabled", BeNil()),
						HaveField("ModuleConfig.Values", MatchAllKeys(Keys{
							"moduleOne": HaveLen(0),
						})),
					)),
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			dataMap := make(map[string]string)
			err := yaml.Unmarshal([]byte(tt.data), &dataMap)
			g.Expect(err).ShouldNot(HaveOccurred(), "should load test data: %s", tt.data)

			l := InitialConfigLoader{}
			kubeCfg, err := l.LegacyConfigMapToInitialConfig(dataMap)
			g.Expect(err).ShouldNot(HaveOccurred(), "should convert data: %s", tt.data)

			tt.matcher(t, kubeCfg)
		})
	}
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
