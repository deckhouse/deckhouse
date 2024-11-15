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
	"testing"

	"github.com/flant/addon-operator/sdk"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestPrepareDeckhouseModuleConfig(t *testing.T) {
	log.InitLogger("simple")

	t.Run("ModuleConfig deckhouse with releaseChannel should remove releaseChannel from mc and adds to result task with returning releaseChannel to post bootstrap tasks", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			config.ModuleConfigGVR: "ModuleConfigList",
		})

		mcWithReleaseChannel := &config.ModuleConfig{}
		mcWithReleaseChannel.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   config.ModuleConfigGroup,
			Version: config.ModuleConfigVersion,
			Kind:    config.ModuleConfigKind,
		})
		mcWithReleaseChannel.SetName("deckhouse")
		mcWithReleaseChannel.Spec.Enabled = ptr.To(true)
		mcWithReleaseChannel.Spec.Version = 1
		mcWithReleaseChannel.Spec.Settings = config.SettingsValues(map[string]interface{}{
			"bundle":         "Minimal",
			"logLevel":       "Debug",
			"releaseChannel": "Alpha",
		})
		res := &ManifestsResult{}
		prepareModuleConfig(mcWithReleaseChannel, res)

		require.NotContains(t, mcWithReleaseChannel.Spec.Settings, "releaseChannel")
		require.Contains(t, mcWithReleaseChannel.Spec.Settings, "bundle")
		require.Contains(t, mcWithReleaseChannel.Spec.Settings, "logLevel")
		require.Equal(t, mcWithReleaseChannel.Spec.Settings["bundle"], "Minimal")
		require.Equal(t, mcWithReleaseChannel.Spec.Settings["logLevel"], "Debug")

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 1)

		u, err := sdk.ToUnstructured(mcWithReleaseChannel)
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

		mcWithoutReleaseChannel := &config.ModuleConfig{}
		mcWithoutReleaseChannel.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   config.ModuleConfigGroup,
			Version: config.ModuleConfigVersion,
			Kind:    config.ModuleConfigKind,
		})
		mcWithoutReleaseChannel.SetName("deckhouse")
		mcWithoutReleaseChannel.Spec.Enabled = ptr.To(true)
		mcWithoutReleaseChannel.Spec.Version = 1
		mcWithoutReleaseChannel.Spec.Settings = config.SettingsValues(map[string]interface{}{
			"bundle": "Minimal",
		})

		res := &ManifestsResult{}
		prepareModuleConfig(mcWithoutReleaseChannel, res)

		require.NotContains(t, mcWithoutReleaseChannel.Spec.Settings, "releaseChannel")
		require.Contains(t, mcWithoutReleaseChannel.Spec.Settings, "bundle")
		require.Equal(t, mcWithoutReleaseChannel.Spec.Settings["bundle"], "Minimal")

		u, err := sdk.ToUnstructured(mcWithoutReleaseChannel)
		require.NoError(t, err)
		_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
		require.NoError(t, err)

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 0)
	})
}

func TestPrepareGlobalModuleConfig(t *testing.T) {
	log.InitLogger("simple")

	t.Run("ModuleConfig global with https setting and another modules settings should remove https from mc and adds to result task with returning https to with resources tasks", func(t *testing.T) {
		t.Run("And keeps another modules settings", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mc := &config.ModuleConfig{}
			mc.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   config.ModuleConfigGroup,
				Version: config.ModuleConfigVersion,
				Kind:    config.ModuleConfigKind,
			})
			mc.SetName("global")
			mc.Spec.Enabled = ptr.To(true)
			mc.Spec.Version = 1
			mc.Spec.Settings = config.SettingsValues(map[string]interface{}{
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
			prepareModuleConfig(mc, res)

			require.Contains(t, mc.Spec.Settings, "modules")
			require.NotContains(t, mc.Spec.Settings["modules"], "https")
			require.True(t, mc.Spec.Settings["highAvailability"].(bool))
			require.Equal(t, mc.Spec.Settings["modules"].(map[string]interface{})["publicDomainTemplate"], "template")

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

			https, found, err := unstructured.NestedMap(resMC.Object, "spec", "settings", "modules", "https")
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, https, map[string]interface{}{
				"customCertificate": map[string]interface{}{
					"secretName": "secret",
				},
			})

			// does not change another fields
			tmpl, found, err := unstructured.NestedString(resMC.Object, "spec", "settings", "modules", "publicDomainTemplate")
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, "template", tmpl)

			ha, found, err := unstructured.NestedBool(resMC.Object, "spec", "settings", "highAvailability")
			require.NoError(t, err)
			require.True(t, found)
			require.True(t, ha)
		})
	})

	t.Run("ModuleConfig global with https setting and without another modules settings should remove https from mc and adds to result task with returning https to with resources tasks", func(t *testing.T) {
		t.Run("And removes modules from settings and keeps another settings", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
				config.ModuleConfigGVR: "ModuleConfigList",
			})

			mc := &config.ModuleConfig{}
			mc.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   config.ModuleConfigGroup,
				Version: config.ModuleConfigVersion,
				Kind:    config.ModuleConfigKind,
			})
			mc.SetName("global")
			mc.Spec.Enabled = ptr.To(true)
			mc.Spec.Version = 1
			mc.Spec.Settings = config.SettingsValues(map[string]interface{}{
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
			prepareModuleConfig(mc, res)

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

			https, found, err := unstructured.NestedMap(resMC.Object, "spec", "settings", "modules", "https")
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, https, map[string]interface{}{
				"customCertificate": map[string]interface{}{
					"secretName": "secret",
				},
			})

			// does not change another fields
			_, found, err = unstructured.NestedString(resMC.Object, "spec", "settings", "modules", "publicDomainTemplate")
			require.NoError(t, err)
			require.False(t, found)

			ha, found, err := unstructured.NestedBool(resMC.Object, "spec", "settings", "highAvailability")
			require.NoError(t, err)
			require.True(t, found)
			require.True(t, ha)
		})
	})

	t.Run("ModuleConfig global without https setting and another modules settings should keep another settings and should not add result task to with resources tasks", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			config.ModuleConfigGVR: "ModuleConfigList",
		})

		mc := &config.ModuleConfig{}
		mc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   config.ModuleConfigGroup,
			Version: config.ModuleConfigVersion,
			Kind:    config.ModuleConfigKind,
		})
		mc.SetName("global")
		mc.Spec.Enabled = ptr.To(true)
		mc.Spec.Version = 1
		mc.Spec.Settings = config.SettingsValues(map[string]interface{}{
			"highAvailability": true,
			"modules": map[string]interface{}{
				"publicDomainTemplate": "template",
			},
		})
		res := &ManifestsResult{}
		prepareModuleConfig(mc, res)

		require.Contains(t, mc.Spec.Settings, "modules")
		require.NotContains(t, mc.Spec.Settings["modules"], "https")
		require.True(t, mc.Spec.Settings["highAvailability"].(bool))
		require.Equal(t, mc.Spec.Settings["modules"].(map[string]interface{})["publicDomainTemplate"], "template")

		require.Len(t, res.WithResourcesMCTasks, 0)
		require.Len(t, res.PostBootstrapMCTasks, 0)

		u, err := sdk.ToUnstructured(mc)
		require.NoError(t, err)
		_, err = fakeClient.Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), u, metav1.CreateOptions{})
		require.NoError(t, err)
	})
}
