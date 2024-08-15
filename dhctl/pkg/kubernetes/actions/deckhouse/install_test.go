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
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestDeckhouseInstall(t *testing.T) {
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	log.InitLogger("simple")
	fakeClient := client.NewFakeKubernetesClient()

	err = os.WriteFile("/deckhouse/version", []byte("1.54.1"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	tests := []struct {
		name    string
		test    func() error
		wantErr bool
	}{
		{
			"Empty config",
			func() error {
				return CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{})
			},
			false,
		},
		{
			"Double install",
			func() error {
				err := CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{})
				if err != nil {
					return err
				}
				return CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{})
			},
			false,
		},
		{
			"With docker cfg",
			func() error {
				err := CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{
					Registry: config.RegistryData{DockerCfg: "YW55dGhpbmc="},
				})
				if err != nil {
					return err
				}
				s, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), "deckhouse-registry", metav1.GetOptions{})
				if err != nil {
					return err
				}

				dockercfg := s.Data[".dockerconfigjson"]
				if string(dockercfg) != "anything" {
					return fmt.Errorf(".dockercfg data: %s", dockercfg)
				}
				return nil
			},
			false,
		},
		{
			"With secrets",
			func() error {
				conf := config.DeckhouseInstaller{
					ClusterConfig:         []byte(`test`),
					ProviderClusterConfig: []byte(``),
					TerraformState:        []byte(`test`),
				}
				err := CreateDeckhouseManifests(fakeClient, &conf)
				if err != nil {
					return err
				}
				return nil
			},
			false,
		},
	}

	for _, tc := range tests {
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
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	fakeClient := client.NewFakeKubernetesClient()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	err = CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{
		DevBranch: "pr1111",
	})

	require.NoError(t, err)
}

func TestDeckhouseInstallWithModuleConfig(t *testing.T) {
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
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
	mc1.Spec.Enabled = pointer.Bool(true)
	mc1.Spec.Version = 1
	mc1.Spec.Settings = config.SettingsValues(map[string]interface{}{
		"ha": true,
	})

	err = CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{
		DevBranch:     "pr1111",
		ModuleConfigs: []*config.ModuleConfig{mc1},
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
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
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
	mc1.Spec.Enabled = pointer.Bool(true)
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
	mc2.Spec.Enabled = pointer.Bool(true)
	mc2.Spec.Version = 1
	mc2.Spec.Settings = config.SettingsValues(map[string]interface{}{
		"bundle": "Minimal",
	})

	err = CreateDeckhouseManifests(fakeClient, &config.DeckhouseInstaller{
		DevBranch:     "pr1111",
		ModuleConfigs: []*config.ModuleConfig{mc1, mc2},
	})

	require.NoError(t, err)

	mcs, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Len(t, mcs.Items, 3)

	require.Equal(t, mcs.Items[0].GetName(), "cni-cilium")
	// should be not found for unlock deckhouse queue
	_, err = fakeClient.CoreV1().ConfigMaps("d8-system").Get(context.TODO(), "deckhouse-bootstrap-lock", metav1.GetOptions{})
	require.True(t, errors.IsNotFound(err))
}
