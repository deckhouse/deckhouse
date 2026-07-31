/*
Copyright 2026 Flant JSC

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

package validation

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

// newControlPlaneManagerConfig builds a control-plane-manager ModuleConfig with the
// given kubernetesVersion setting (omitted from settings entirely when empty).
func newControlPlaneManagerConfig(kubernetesVersion string) *v1alpha1.ModuleConfig {
	cfg := newModuleConfigFull(controlPlaneManagerModuleName, boolPtr(true), "", "")
	cfg.Spec.Version = 1
	settings := map[string]any{}
	if kubernetesVersion != "" {
		settings["kubernetesVersion"] = kubernetesVersion
	}
	cfg.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	return cfg
}

// newControlPlaneManagerConfigDisabled is like newControlPlaneManagerConfig but with
// enabled=false so DELETE skips the confirmation guard and reaches the version check.
func newControlPlaneManagerConfigDisabled(kubernetesVersion string) *v1alpha1.ModuleConfig {
	cfg := newControlPlaneManagerConfig(kubernetesVersion)
	cfg.Spec.Enabled = boolPtr(false)
	return cfg
}

func newClusterConfigurationSecret(kubernetesVersion string) *corev1.Secret {
	raw := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nkubernetesVersion: \"" + kubernetesVersion + "\"\n"
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte(raw),
		},
	}
}

// newClusterConfigurationSecretWithMaxUsed also carries the version bookkeeping
// control-plane-manager keeps alongside the ClusterConfiguration document.
func newClusterConfigurationSecretWithMaxUsed(kubernetesVersion, maxUsed string) *corev1.Secret {
	secret := newClusterConfigurationSecret(kubernetesVersion)
	secret.Data["maxUsedControlPlaneKubernetesVersion"] = []byte(maxUsed)
	return secret
}

func newClusterKubernetesConfigMap(available []string) *corev1.ConfigMap {
	var status strings.Builder
	status.WriteString("availableVersions:\n")
	for _, v := range available {
		status.WriteString("  - \"")
		status.WriteString(v)
		status.WriteString("\"\n")
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-kubernetes", Namespace: "kube-system"},
		Data: map[string]string{
			"status": status.String(),
		},
	}
}

func TestModuleConfigValidationHandler_ControlPlaneManagerKubernetesVersion(t *testing.T) {
	const moduleName = controlPlaneManagerModuleName

	// spec.version=1 with settings is exercised in these tests, which requires a
	// non-nil conversions store (the default nil-nil validator panics on Get).
	validator := configtools.NewValidator(nil, conversion.NewConversionsStore())

	buildHandler := func(t *testing.T) (storage *fakeModuleStorage, manager *fakeModuleManager) {
		t.Helper()
		storage = &fakeModuleStorage{
			modules: map[string]*moduletypes.Module{
				moduleName: newStorageModule(t, moduleName, "", ""),
			},
		}
		manager = &fakeModuleManager{enabled: map[string]bool{moduleName: true}}
		return storage, manager
	}

	defaultAvailable := []string{"1.33", "1.34", "1.35", "1.36"}

	withObjs := func(t *testing.T, objs ...client.Object) http.Handler {
		t.Helper()
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		all := append([]client.Object{moduleCR}, objs...)
		return newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, all...)
	}

	t.Run("HV-07: no kubernetesVersion in new settings — allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(defaultAvailable))

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("HV-06: Automatic is allowed without membership check", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap([]string{"1.34", "1.35"}))

		newCfg := newControlPlaneManagerConfig("Automatic")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("HV-05: upgrade to a version in availableVersions is allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(defaultAvailable))

		newCfg := newControlPlaneManagerConfig("1.35")
		oldCfg := newControlPlaneManagerConfig("1.33")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("HV-03/HV-04: version at maxUsed-1 is allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(defaultAvailable))

		newCfg := newControlPlaneManagerConfig("1.33")
		oldCfg := newControlPlaneManagerConfig("1.34")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("HV-02: version below availableVersions is rejected", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(defaultAvailable))

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "not in the cluster's availableVersions")
	})

	t.Run("HV-08: version below maxUsed-1 is rejected", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}))

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "not in the cluster's availableVersions")
	})

	t.Run("п.2: clearing MC override falls back to stale CC pin and is rejected", func(t *testing.T) {
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
		assert.Contains(t, resp.Result.Message, "1.32")
		assert.Contains(t, resp.Result.Message, "not in the cluster's availableVersions")
	})

	t.Run("п.2: clearing MC override when CC is in availableVersions is allowed", func(t *testing.T) {
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.34"),
		)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("п.2: clearing MC override to Automatic CC is allowed", func(t *testing.T) {
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("Automatic"),
		)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("switching a pin to Automatic is allowed and ignores a stale CC pin", func(t *testing.T) {
		// Automatic hands the choice back to Deckhouse; the resolver no longer consults CC when
		// the setting is present, and the Automatic path cannot drop below maxUsed-1 anyway.
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		newCfg := newControlPlaneManagerConfig("Automatic")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("dropping the setting from Automatic is checked against the CC fallback", func(t *testing.T) {
		// Removing the field does change ownership back to ClusterConfiguration, so a stale pin
		// there must not silently become the target.
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("Automatic")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
		assert.Contains(t, resp.Result.Message, "1.32")
	})

	t.Run("clear still rejected when old settings extract fails", func(t *testing.T) {
		handler := withObjs(t,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		// Unsupported version makes ExtractLatestSettings fail; the clear-guard must still
		// see the raw kubernetesVersion pin via GetMap fallback.
		oldCfg.Spec.Version = 99
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
		assert.Contains(t, resp.Result.Message, "1.32")
	})

	t.Run("unchanged kubernetesVersion skips membership — other fields editable", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}))

		// Pin 1.32 is outside availableVersions; an unrelated settings edit must still pass.
		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("1.32")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("DELETE with pinned version falls back to stale CC and is rejected", func(t *testing.T) {
		storage, manager := buildHandler(t)
		// Module must not be treated as enabled so confirmation does not fire first.
		manager.enabled[moduleName] = false
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		oldCfg := newControlPlaneManagerConfigDisabled("1.35")
		review := newModuleConfigAdmissionReview("DELETE", nil, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
		assert.Contains(t, resp.Result.Message, "not in the cluster's availableVersions")
	})

	t.Run("DELETE with allow-disabling annotation still checks version fallback", func(t *testing.T) {
		storage, manager := buildHandler(t)
		manager.enabled[moduleName] = true
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		oldCfg := newControlPlaneManagerConfig("1.35")
		if oldCfg.Annotations == nil {
			oldCfg.Annotations = map[string]string{}
		}
		oldCfg.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable] = "true"
		review := newModuleConfigAdmissionReview("DELETE", nil, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
	})

	t.Run("DELETE still rejected when old settings extract would fail", func(t *testing.T) {
		storage, manager := buildHandler(t)
		manager.enabled[moduleName] = false
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR,
			newClusterKubernetesConfigMap([]string{"1.34", "1.35", "1.36"}),
			newClusterConfigurationSecret("1.32"),
		)

		oldCfg := newControlPlaneManagerConfigDisabled("1.35")
		oldCfg.Spec.Version = 99
		review := newModuleConfigAdmissionReview("DELETE", nil, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "1.32")
	})

	t.Run("fail-open: no ConfigMap and no maxUsed baseline — allowed", func(t *testing.T) {
		handler := withObjs(t)

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	// Before update-observer publishes status.availableVersions — a fresh cluster, a recreated
	// ConfigMap, an empty status — the membership check has nothing to work with. The Secret
	// bookkeeping control-plane-manager has always maintained takes over there, so a deep
	// downgrade cannot slip through that window.
	t.Run("no ConfigMap: pin more than one minor below maxUsed is rejected", func(t *testing.T) {
		handler := withObjs(t, newClusterConfigurationSecretWithMaxUsed("1.35", "1.35"))

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "more than one minor below")
		assert.Contains(t, resp.Result.Message, "1.35")
	})

	t.Run("no ConfigMap: pin exactly at maxUsed-1 is allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterConfigurationSecretWithMaxUsed("1.35", "1.35"))

		newCfg := newControlPlaneManagerConfig("1.34")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("no ConfigMap: upgrade above maxUsed is allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterConfigurationSecretWithMaxUsed("1.34", "1.34"))

		newCfg := newControlPlaneManagerConfig("1.36")
		oldCfg := newControlPlaneManagerConfig("1.34")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("no ConfigMap: clearing the override onto a stale CC pin is rejected via maxUsed", func(t *testing.T) {
		handler := withObjs(t, newClusterConfigurationSecretWithMaxUsed("1.32", "1.35"))

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "ClusterConfiguration.kubernetesVersion")
		assert.Contains(t, resp.Result.Message, "more than one minor below")
	})

	t.Run("empty availableVersions falls through to the maxUsed guard", func(t *testing.T) {
		handler := withObjs(t,
			newClusterKubernetesConfigMap(nil),
			newClusterConfigurationSecretWithMaxUsed("1.35", "1.35"),
		)

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "more than one minor below")
	})

	t.Run("fail-open: empty availableVersions — allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(nil))

		newCfg := newControlPlaneManagerConfig("1.32")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("fail-open: clearing override with no Secret — allowed", func(t *testing.T) {
		handler := withObjs(t, newClusterKubernetesConfigMap(defaultAvailable))

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})
}

// TestModuleConfigOwnsKubernetesVersion covers the predicate that lets the ClusterConfiguration
// webhook stand down. Presence of the setting — not its value — decides which document owns the
// version, so an explicit "Automatic" counts as ownership just like a pin does.
//
// Getting this wrong is not cosmetic: while the ModuleConfig owns the version, the leftover
// ClusterConfiguration field describes nothing the cluster will actually run, and validating it
// blocks edits (including the field's own removal, the last step of the documented migration).
func TestModuleConfigOwnsKubernetesVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	newClient := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	}

	t.Run("no ModuleConfig — ClusterConfiguration still owns the version", func(t *testing.T) {
		assert.False(t, moduleConfigOwnsKubernetesVersion(context.Background(), newClient()))
	})

	t.Run("ModuleConfig without settings does not claim ownership", func(t *testing.T) {
		cfg := newControlPlaneManagerConfig("")
		assert.False(t, moduleConfigOwnsKubernetesVersion(context.Background(), newClient(cfg)))
	})

	t.Run("pinned ModuleConfig owns the version", func(t *testing.T) {
		cfg := newControlPlaneManagerConfig("1.35")
		assert.True(t, moduleConfigOwnsKubernetesVersion(context.Background(), newClient(cfg)))
	})

	t.Run("explicit Automatic owns the version too", func(t *testing.T) {
		cfg := newControlPlaneManagerConfig("Automatic")
		assert.True(t, moduleConfigOwnsKubernetesVersion(context.Background(), newClient(cfg)))
	})

	t.Run("a different module's ModuleConfig is irrelevant", func(t *testing.T) {
		cfg := newControlPlaneManagerConfig("1.35")
		cfg.SetName("node-manager")
		assert.False(t, moduleConfigOwnsKubernetesVersion(context.Background(), newClient(cfg)))
	})
}
