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

package deckhouse

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	registry_types "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func createRegistryDefaultConfig(moduleEnable bool) registry_config.Config {
	return registry_config.Config{
		ModuleEnabled: moduleEnable,
		Mode: &registry_config.UnmanagedMode{
			Remote: registry_types.Data{
				ImagesRepo: registry_config.DefaultImagesRepo,
				Scheme:     registry_config.DefaultScheme,
			},
		},
	}
}

func TestDeckhouseInstall(t *testing.T) {
	ctx := context.Background()
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	err = os.Setenv("DHCTL_TEST_VERSION_TAG", "1.54.1")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
		os.Unsetenv("DHCTL_TEST_VERSION_TAG")
	}()

	log.InitLogger("json")
	fakeClient := client.NewFakeKubernetesClient()

	tests := []struct {
		name    string
		test    func() error
		wantErr bool
	}{
		{
			"Empty config",
			func() error {
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(false),
				}, func() error {
					return nil
				})
				return err
			},
			false,
		},
		{
			"Double install",
			func() error {
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(false),
				}, func() error {
					return nil
				})
				if err != nil {
					return err
				}
				_, err = CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(false),
				}, func() error {
					return nil
				})
				return err
			},
			false,
		},
		{
			"With docker cfg",
			func() error {
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(false),
				}, func() error {
					return nil
				})
				if err != nil {
					return err
				}
				s, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), "deckhouse-registry", metav1.GetOptions{})
				if err != nil {
					return err
				}

				dockercfg := s.Data[".dockerconfigjson"]
				if string(dockercfg) == "" {
					return fmt.Errorf("empty dockercfg in deckhouse-registry secret")
				}
				return nil
			},
			false,
		},
		{
			"With bashible cfg",
			func() error {
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(true),
				}, func() error {
					return nil
				})
				if err != nil {
					return err
				}
				s, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), "registry-bashible-config", metav1.GetOptions{})
				if err != nil {
					return err
				}
				config := s.Data["config"]
				if string(config) == "" {
					return fmt.Errorf("empty config in registry-bashible-config secret")
				}
				return nil
			},
			false,
		},
		{
			"Without bashible cfg",
			func() error {
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
					Registry: createRegistryDefaultConfig(false),
				}, func() error {
					return nil
				})
				if err != nil {
					return err
				}
				_, err = fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), "registry-bashible-config", metav1.GetOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
				return nil
			},
			false,
		},
		{
			"With secrets",
			func() error {
				conf := config.DeckhouseInstaller{
					Registry:              createRegistryDefaultConfig(false),
					ClusterConfig:         []byte(`test`),
					ProviderClusterConfig: []byte(`test`),
					InfrastructureState:   []byte(`test`),
				}
				_, err := CreateDeckhouseManifests(ctx, fakeClient, &conf, func() error {
					return nil
				})
				if err != nil {
					return err
				}
				return nil
			},
			false,
		},
	}

	for _, tc := range tests {
		fmt.Printf("Running test case: %s\n", tc.name)
		err := tc.test()

		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}

func TestDeckhouseInstallWithDevBranch(t *testing.T) {
	ctx := context.Background()
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	err = os.Setenv("DHCTL_TEST_VERSION_TAG", "dev")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
		os.Unsetenv("DHCTL_TEST_VERSION_TAG")
	}()

	fakeClient := client.NewFakeKubernetesClient()

	_, err = CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
		Registry:  createRegistryDefaultConfig(false),
		DevBranch: "pr1111",
	}, func() error {
		return nil
	})

	require.NoError(t, err)
}

func TestDeckhouseInstallWithModuleConfig(t *testing.T) {
	ctx := context.Background()
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	err = os.Setenv("DHCTL_TEST_VERSION_TAG", "dev")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
		os.Unsetenv("DHCTL_TEST_VERSION_TAG")
	}()

	fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		config.ModuleConfigGVR: "ModuleConfigList",
	})

	mc1 := &config.ModuleConfig{}
	mc1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   config.ModuleConfigGroup,
		Version: config.ModuleConfigVersion,
		Kind:    config.ModuleConfigKind,
	})
	mc1.SetName("global")
	mc1.Spec.Enabled = ptr.To(true)
	mc1.Spec.Version = 1
	mc1.Spec.Settings = config.SettingsValues(map[string]interface{}{
		"ha": true,
	})

	_, err = CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
		Registry:      createRegistryDefaultConfig(false),
		DevBranch:     "pr1111",
		ModuleConfigs: []*config.ModuleConfig{mc1},
	}, func() error {
		return nil
	})

	require.NoError(t, err)

	mc, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "global", metav1.GetOptions{})
	require.NoError(t, err)

	require.NotNil(t, mc)

	// should be not found for unlock deckhouse queue
	_, err = fakeClient.CoreV1().ConfigMaps("d8-system").Get(context.TODO(), "deckhouse-bootstrap-lock", metav1.GetOptions{})
	require.True(t, errors.IsNotFound(err))
}

func TestDeckhouseInstallWithModuleConfigs(t *testing.T) {
	ctx := context.Background()
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	err = os.Setenv("DHCTL_TEST_VERSION_TAG", "dev")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
		os.Unsetenv("DHCTL_TEST_VERSION_TAG")
	}()

	fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		config.ModuleConfigGVR: "ModuleConfigList",
	})

	mc1 := &config.ModuleConfig{}
	mc1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   config.ModuleConfigGroup,
		Version: config.ModuleConfigVersion,
		Kind:    config.ModuleConfigKind,
	})
	mc1.SetName("global")
	mc1.Spec.Enabled = ptr.To(true)
	mc1.Spec.Version = 1
	mc1.Spec.Settings = config.SettingsValues(map[string]interface{}{
		"ha": true,
	})

	mc2 := &config.ModuleConfig{}
	mc2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   config.ModuleConfigGroup,
		Version: config.ModuleConfigVersion,
		Kind:    config.ModuleConfigKind,
	})
	mc2.SetName("deckhouse")
	mc2.Spec.Enabled = ptr.To(true)
	mc2.Spec.Version = 1
	mc2.Spec.Settings = config.SettingsValues(map[string]interface{}{
		"bundle": "Minimal",
	})

	_, err = CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
		Registry:      createRegistryDefaultConfig(false),
		DevBranch:     "pr1111",
		ModuleConfigs: []*config.ModuleConfig{mc1, mc2},
	}, func() error {
		return nil
	})

	require.NoError(t, err)

	mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Len(t, mcs.Items, 2)

	// should be not found for unlock deckhouse queue
	_, err = fakeClient.CoreV1().ConfigMaps("d8-system").Get(context.TODO(), "deckhouse-bootstrap-lock", metav1.GetOptions{})
	require.True(t, errors.IsNotFound(err))
}

func TestDeckhouseInstallWithModuleConfigsReturnsResults(t *testing.T) {
	ctx := context.Background()
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	err = os.Setenv("DHCTL_TEST_VERSION_TAG", "dev")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
		os.Unsetenv("DHCTL_TEST_VERSION_TAG")
	}()

	t.Run("Only deckhouse mc", func(t *testing.T) {
		t.Run("Should create only one post bootstrap mc task", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mc := createMC("deckhouse", map[string]interface{}{
				"bundle":         "Minimal",
				"logLevel":       "Debug",
				"releaseChannel": "Alpha",
			})

			res, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
				Registry:      createRegistryDefaultConfig(false),
				DevBranch:     "pr1111",
				ModuleConfigs: []*config.ModuleConfig{mc},
			}, func() error {
				return nil
			})
			require.NoError(t, err)

			require.Len(t, res.WithResourcesMCTasks, 0)
			require.Len(t, res.PostBootstrapMCTasks, 1)
			require.Equal(t, res.PostBootstrapMCTasks[0].Title, "Set release channel to deckhouse module config")

			mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			require.Len(t, mcs.Items, 1)

			require.NotContains(t, mcs.Items[0].Object["spec"].(map[string]interface{})["settings"], "releaseChannel")
		})
	})

	t.Run("Only global mcs", func(t *testing.T) {
		t.Run("Should create with resources tasks only one", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mc := createMC("global", map[string]interface{}{
				"highAvailability": true,
				"modules": map[string]interface{}{
					"https": map[string]interface{}{
						"customCertificate": map[string]interface{}{
							"secretName": "secret",
						},
					},
					"publicDomainTemplate": "template",
				},
			})

			res, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
				Registry:      createRegistryDefaultConfig(false),
				DevBranch:     "pr1111",
				ModuleConfigs: []*config.ModuleConfig{mc},
			}, func() error {
				return nil
			})
			require.NoError(t, err)

			require.Len(t, res.WithResourcesMCTasks, 1)
			require.Len(t, res.PostBootstrapMCTasks, 0)
			require.Equal(t, res.WithResourcesMCTasks[0].Title, "Set https setting to global module config")

			mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			require.Len(t, mcs.Items, 1)

			require.NotContains(t, mcs.Items[0].Object["spec"].(map[string]interface{})["settings"].(map[string]interface{})["modules"], "https")
		})
	})

	t.Run("Without global and deckhouse mcs", func(t *testing.T) {
		t.Run("Should create with resources tasks only one", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mc := createMC("prometheus", map[string]interface{}{
				"highAvailability": true,
			})

			res, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
				Registry:      createRegistryDefaultConfig(false),
				DevBranch:     "pr1111",
				ModuleConfigs: []*config.ModuleConfig{mc},
			}, func() error {
				return nil
			})
			require.NoError(t, err)

			require.Len(t, res.WithResourcesMCTasks, 0)
			require.Len(t, res.PostBootstrapMCTasks, 0)

			mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			require.Len(t, mcs.Items, 1)
		})
	})

	t.Run("Deckhouse + global mcs", func(t *testing.T) {
		t.Run("Should create post bootstrap and with resources mc task", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mcDeckhouse := createMC("deckhouse", map[string]interface{}{
				"bundle":         "Minimal",
				"logLevel":       "Debug",
				"releaseChannel": "Alpha",
			})

			mcGlobal := createMC("global", map[string]interface{}{
				"highAvailability": true,
				"modules": map[string]interface{}{
					"https": map[string]interface{}{
						"customCertificate": map[string]interface{}{
							"secretName": "secret",
						},
					},
				},
			})

			res, err := CreateDeckhouseManifests(ctx, fakeClient, &config.DeckhouseInstaller{
				Registry:      createRegistryDefaultConfig(false),
				DevBranch:     "pr1111",
				ModuleConfigs: []*config.ModuleConfig{mcDeckhouse, mcGlobal},
			}, func() error {
				return nil
			})
			require.NoError(t, err)

			require.Len(t, res.WithResourcesMCTasks, 1)
			require.Len(t, res.PostBootstrapMCTasks, 1)
			require.Equal(t, res.PostBootstrapMCTasks[0].Title, "Set release channel to deckhouse module config")
			require.Equal(t, res.WithResourcesMCTasks[0].Title, "Set https setting to global module config")

			mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			require.Len(t, mcs.Items, 2)
		})
	})
}
