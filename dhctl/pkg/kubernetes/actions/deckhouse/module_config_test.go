// Copyright 2024 Flant JSC
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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func createMC(name string, settings map[string]interface{}) *config.ModuleConfig {
	mc := &config.ModuleConfig{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   config.ModuleConfigGroup,
		Version: config.ModuleConfigVersion,
		Kind:    config.ModuleConfigKind,
	})
	mc.SetName(name)
	mc.Spec.Enabled = ptr.To(true)
	mc.Spec.Version = 1
	mc.Spec.Settings = config.SettingsValues(settings)

	return mc
}

func TestPrepareDeckhouseModuleConfig(t *testing.T) {
	ctx := context.Background()
	log.InitLogger("json")

	t.Run("ModuleConfig deckhouse with releaseChannel should remove releaseChannel from mc and adds to result task with returning releaseChannel to post bootstrap tasks", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			config.ModuleConfigGVR: "ModuleConfigList",
		})

		mc := createMC("deckhouse", map[string]interface{}{
			"bundle":         "Minimal",
			"logLevel":       "Debug",
			"releaseChannel": "Alpha",
		})

		res := &ManifestsResult{}
		prepareModuleConfig(ctx, mc, res)

		require.NotContains(t, mc.Spec.Settings, "releaseChannel")
		require.Contains(t, mc.Spec.Settings, "bundle")
		require.Contains(t, mc.Spec.Settings, "logLevel")
		require.Equal(t, mc.Spec.Settings["bundle"], "Minimal")
		require.Equal(t, mc.Spec.Settings["logLevel"], "Debug")

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 1)

		u, err := sdk.ToUnstructured(mc)
		require.NoError(t, err)
		_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
		require.NoError(t, err)

		require.Equal(t, res.PostBootstrapMCTasks[0].Title, "Set release channel to deckhouse module config")

		err = res.PostBootstrapMCTasks[0].Do(fakeClient)
		require.NoError(t, err)

		resMC, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "deckhouse", metav1.GetOptions{})
		require.NoError(t, err)

		rc, found, err := unstructured.NestedString(resMC.Object, "spec", "settings", "releaseChannel")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "Alpha", rc)

		// does not change another fields
		lg, found, err := unstructured.NestedString(resMC.Object, "spec", "settings", "logLevel")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "Debug", lg)

		bundle, found, err := unstructured.NestedString(resMC.Object, "spec", "settings", "bundle")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "Minimal", bundle)
	})

	t.Run("ModuleConfig deckhouse without releaseChannel should keep as is mc and should not add tasks", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			config.ModuleConfigGVR: "ModuleConfigList",
		})

		mc := createMC("deckhouse", map[string]interface{}{
			"bundle": "Minimal",
		})

		res := &ManifestsResult{}
		prepareModuleConfig(ctx, mc, res)

		require.NotContains(t, mc.Spec.Settings, "releaseChannel")
		require.Contains(t, mc.Spec.Settings, "bundle")
		require.Equal(t, mc.Spec.Settings["bundle"], "Minimal")

		u, err := sdk.ToUnstructured(mc)
		require.NoError(t, err)
		_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
		require.NoError(t, err)

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 0)
	})
}

func TestPrepareGlobalModuleConfig(t *testing.T) {
	ctx := context.Background()
	log.InitLogger("json")

	assertSaveAnotherFields := func(t *testing.T, mc *unstructured.Unstructured, publicDomainTemplateFound bool) {
		// does not change another fields
		ha, found, err := unstructured.NestedBool(mc.Object, "spec", "settings", "highAvailability")
		require.NoError(t, err)
		require.True(t, found)
		require.True(t, ha)

		tmpl, found, err := unstructured.NestedString(mc.Object, "spec", "settings", "modules", "publicDomainTemplate")
		require.NoError(t, err)

		if publicDomainTemplateFound {
			require.True(t, found)
			require.Equal(t, "template", tmpl)
			return
		}

		require.False(t, found)
	}

	assertHTTPSSettings := func(t *testing.T, mc *unstructured.Unstructured) {
		https, found, err := unstructured.NestedMap(mc.Object, "spec", "settings", "modules", "https")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, https, map[string]interface{}{
			"customCertificate": map[string]interface{}{
				"secretName": "secret",
			},
		})
	}

	assertMCAfterPrepare := func(t *testing.T, mc *config.ModuleConfig) {
		require.Contains(t, mc.Spec.Settings, "modules")
		require.NotContains(t, mc.Spec.Settings["modules"], "https")
		require.True(t, mc.Spec.Settings["highAvailability"].(bool))
		require.Equal(t, mc.Spec.Settings["modules"].(map[string]interface{})["publicDomainTemplate"], "template")
	}

	t.Run("ModuleConfig global with https setting and another modules settings should remove https from mc and adds to result task with returning https to with resources tasks", func(t *testing.T) {
		t.Run("And keeps another modules settings", func(t *testing.T) {
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

			res := &ManifestsResult{}
			prepareModuleConfig(ctx, mc, res)

			assertMCAfterPrepare(t, mc)

			require.Len(t, res.WithResourcesMCTasks, 1)
			require.Len(t, res.PostBootstrapMCTasks, 0)

			u, err := sdk.ToUnstructured(mc)
			require.NoError(t, err)
			_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
			require.NoError(t, err)

			require.Equal(t, res.WithResourcesMCTasks[0].Title, "Set https setting to global module config")

			err = res.WithResourcesMCTasks[0].Do(fakeClient)
			require.NoError(t, err)

			resMC, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "global", metav1.GetOptions{})
			require.NoError(t, err)

			assertHTTPSSettings(t, resMC)

			assertSaveAnotherFields(t, resMC, true)
		})
	})

	t.Run("ModuleConfig global with https setting and without another modules settings should remove https from mc and adds to result task with returning https to with resources tasks", func(t *testing.T) {
		t.Run("And removes modules from settings and keeps another settings", func(t *testing.T) {
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
				},
			})

			res := &ManifestsResult{}
			prepareModuleConfig(ctx, mc, res)

			require.NotContains(t, mc.Spec.Settings, "modules")
			require.True(t, mc.Spec.Settings["highAvailability"].(bool))

			require.Len(t, res.WithResourcesMCTasks, 1)
			require.Len(t, res.PostBootstrapMCTasks, 0)

			u, err := sdk.ToUnstructured(mc)
			require.NoError(t, err)
			_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
			require.NoError(t, err)

			require.Equal(t, res.WithResourcesMCTasks[0].Title, "Set https setting to global module config")

			err = res.WithResourcesMCTasks[0].Do(fakeClient)
			require.NoError(t, err)

			resMC, err := fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "global", metav1.GetOptions{})
			require.NoError(t, err)

			assertHTTPSSettings(t, resMC)

			assertSaveAnotherFields(t, resMC, false)
		})
	})

	t.Run("ModuleConfig global without https setting and another modules settings should keep another settings and should not add result task to with resources tasks", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			config.ModuleConfigGVR: "ModuleConfigList",
		})

		mc := createMC("global", map[string]interface{}{
			"highAvailability": true,
			"modules": map[string]interface{}{
				"publicDomainTemplate": "template",
			},
		})

		res := &ManifestsResult{}
		prepareModuleConfig(ctx, mc, res)

		assertMCAfterPrepare(t, mc)

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 0)

		u, err := sdk.ToUnstructured(mc)
		require.NoError(t, err)
		_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
		require.NoError(t, err)
	})
}
