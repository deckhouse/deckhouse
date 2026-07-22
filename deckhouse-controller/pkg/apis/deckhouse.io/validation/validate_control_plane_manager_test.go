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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func newClusterConfigurationSecret(kubernetesVersion, maxUsedControlPlaneKubernetesVersion string) *corev1.Secret {
	yaml := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nkubernetesVersion: \"" + kubernetesVersion + "\"\n"
	data := map[string][]byte{
		"cluster-configuration.yaml": []byte(yaml),
	}
	if maxUsedControlPlaneKubernetesVersion != "" {
		data["maxUsedControlPlaneKubernetesVersion"] = []byte(maxUsedControlPlaneKubernetesVersion)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data:       data,
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

	t.Run("no kubernetesVersion in new settings — not guarded", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		secret := newClusterConfigurationSecret("1.35", "1.35")
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR, secret)

		newCfg := newControlPlaneManagerConfig("")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("upgrade from ClusterConfiguration baseline is allowed", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		secret := newClusterConfigurationSecret("1.33", "1.33")
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR, secret)

		newCfg := newControlPlaneManagerConfig("1.35")
		oldCfg := newControlPlaneManagerConfig("") // no prior MC override — falls back to CC
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("downgrade more than 1 minor below ClusterConfiguration baseline is rejected", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		secret := newClusterConfigurationSecret("1.35", "1.35")
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR, secret)

		newCfg := newControlPlaneManagerConfig("1.33")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "can not downgrade kubernetes version")
	})

	t.Run("downgrade more than 1 minor below a prior explicit MC version is rejected even if ClusterConfiguration is stale", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		// ClusterConfiguration is stale/lower than what the cluster actually runs;
		// maxUsedControlPlaneKubernetesVersion (written by effective_kubernetes_version.go)
		// reflects the real floor.
		secret := newClusterConfigurationSecret("1.30", "1.36")
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR, secret)

		newCfg := newControlPlaneManagerConfig("1.33")
		oldCfg := newControlPlaneManagerConfig("1.36")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "can not downgrade kubernetes version")
	})

	t.Run("downgrade within 1 minor is allowed", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		secret := newClusterConfigurationSecret("1.35", "1.35")
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR, secret)

		newCfg := newControlPlaneManagerConfig("1.34")
		oldCfg := newControlPlaneManagerConfig("1.35")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("no d8-cluster-configuration secret yet — not guarded", func(t *testing.T) {
		storage, manager := buildHandler(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR)

		newCfg := newControlPlaneManagerConfig("1.33")
		oldCfg := newControlPlaneManagerConfig("")
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})
}
