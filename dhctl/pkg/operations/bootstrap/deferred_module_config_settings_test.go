// Copyright 2026 Flant JSC
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

package bootstrap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func newModuleConfig(name string, settings map[string]any) *config.ModuleConfig {
	mc := &config.ModuleConfig{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   config.ModuleConfigGroup,
		Version: config.ModuleConfigVersion,
		Kind:    config.ModuleConfigKind,
	})
	mc.SetName(name)
	mc.Spec.Enabled = new(true)
	mc.Spec.Version = 1
	mc.Spec.Settings = config.SettingsValues(settings)

	return mc
}

func TestInstallDeckhouseAndApplyDeferredSettingsRestoresDeferredSettings(t *testing.T) {
	t.Setenv("DHCTL_TEST", "yes")
	t.Setenv("DHCTL_TEST_VERSION_TAG", "dev")

	ctx := t.Context()
	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		config.ModuleConfigGVR: "ModuleConfigList",
	})

	_, err := kubeCl.CoreV1().Pods("d8-system").Create(ctx, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse-pod",
			Namespace: "d8-system",
			Labels:    map[string]string{"app": "deckhouse"},
		},
		Status: v1.PodStatus{
			Phase:      v1.PodRunning,
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	https := map[string]any{"mode": "CustomCertificate", "customCertificate": map[string]any{"secretName": "cert"}}
	err = installDeckhouseAndApplyDeferredSettings(ctx, kubeCl, &config.DeckhouseInstaller{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithModeUnmanaged(),
			registry_mocks.WithLegacyMode(),
		),
		DevBranch: "pr1111",
		ModuleConfigs: []*config.ModuleConfig{
			newModuleConfig("deckhouse", map[string]any{"releaseChannel": "Alpha"}),
			newModuleConfig("global", map[string]any{"modules": map[string]any{"https": https}}),
		},
	}, InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error { return nil },
		DeckhouseTimeout:    15 * time.Minute,
	})
	require.NoError(t, err)

	get := func(name string) *unstructured.Unstructured {
		mc, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Get(ctx, name, metav1.GetOptions{})
		require.NoError(t, err)

		return mc
	}

	channel, _, err := unstructured.NestedString(get("deckhouse").Object, "spec", "settings", "releaseChannel")
	require.NoError(t, err)
	require.Equal(t, "Alpha", channel)

	restoredHTTPS, _, err := unstructured.NestedMap(get("global").Object, "spec", "settings", "modules", "https")
	require.NoError(t, err)
	require.Equal(t, https, restoredHTTPS)
}
