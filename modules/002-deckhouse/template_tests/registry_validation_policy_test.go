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

package template_tests

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// This test covers the deckhouse-module-registry-validation policy: the registry modes Proxy and
// Local of the previous implementation cannot be written to the `deckhouse` ModuleConfig, whether
// the ModuleConfig is created with them or edited to them. A value already stored is not rewritten,
// so edits that keep it pass. The expressions are compiled out of the
// chart template and evaluated with the harness in validation_policy_test.go.

func TestRegistryModesOfThePreviousImplementationAreRefused(t *testing.T) {
	policy := loadPolicy(t, "templates/registry/validation.yaml", "deckhouse-mc")

	tests := []struct {
		name      string
		request   object
		oldObject any
		object    any
		want      policyDecision
		detail    string
	}{
		// -- deckhouse-mc -------------------------------------------------------
		{
			name:    "another module's ModuleConfig is out of scope",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("registry", object{"mode": "Unmanaged"}),
			want:    skipped,
			detail:  "deckhouse-mc",
		},

		// -- validations --------------------------------------------------------
		{
			name:    "creating it with Proxy is refused",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("deckhouse", registrySettings("Proxy")),
			want:    denied,
			detail:  "previous implementation",
		},
		{
			name:    "creating it with Local is refused",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("deckhouse", registrySettings("Local")),
			want:    denied,
			detail:  "previous implementation",
		},
		{
			name:      "switching to Proxy is refused",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", registrySettings("Unmanaged")),
			object:    moduleConfig("deckhouse", registrySettings("Proxy")),
			want:      denied,
			detail:    "`registry` ModuleConfig",
		},
		{
			name:      "switching to Local is refused",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", registrySettings("Direct")),
			object:    moduleConfig("deckhouse", registrySettings("Local")),
			want:      denied,
			detail:    "`registry` ModuleConfig",
		},
		{
			name:      "switching from Proxy to Local is refused",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", registrySettings("Proxy")),
			object:    moduleConfig("deckhouse", registrySettings("Local")),
			want:      denied,
			detail:    "`Unmanaged`",
		},
		{
			// A stored Proxy can only have got there before this policy existed. Refusing every edit
			// that keeps it would block unrelated settings and labels too.
			name:      "an edit that keeps a stored Proxy is allowed",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", object{"registry": object{"mode": "Proxy"}}),
			object:    moduleConfig("deckhouse", object{"registry": object{"mode": "Proxy"}, "logLevel": "Debug"}),
			want:      allowed,
		},
		{
			// Deleting the ModuleConfig ends in an UPDATE by deckhouse-controller that removes its
			// finalizer. Refused, the ModuleConfig would stay in Terminating.
			name:      "removing the finalizer of a ModuleConfig with a stored Local is allowed",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: withFinalizers(moduleConfig("deckhouse", registrySettings("Local")), "modules.deckhouse.io/module-registered"),
			object:    withFinalizers(moduleConfig("deckhouse", registrySettings("Local"))),
			want:      allowed,
		},
		{
			name:      "leaving Proxy for Unmanaged is allowed",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", registrySettings("Proxy")),
			object:    moduleConfig("deckhouse", registrySettings("Unmanaged")),
			want:      allowed,
		},
		{
			// The fix the D8DeckhouseRegistryModeUnsupported alert gives for a stored value.
			name:      "removing the registry section with a stored Local is allowed",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", registrySettings("Local")),
			object:    moduleConfig("deckhouse", object{"bundle": "Default"}),
			want:      allowed,
		},
		{
			name:    "Unmanaged is allowed",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("deckhouse", registrySettings("Unmanaged")),
			want:    allowed,
		},
		{
			name:    "Direct is allowed",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("deckhouse", registrySettings("Direct")),
			want:    allowed,
		},
		{
			name:    "a ModuleConfig without the registry section is allowed",
			request: moduleConfigRequest("CREATE"),
			object:  moduleConfig("deckhouse", object{"bundle": "Default"}),
			want:    allowed,
		},
		{
			name:    "a ModuleConfig without settings is allowed",
			request: moduleConfigRequest("CREATE"),
			object:  object{"apiVersion": "deckhouse.io/v1alpha1", "kind": "ModuleConfig", "metadata": object{"name": "deckhouse"}, "spec": object{"version": 1}},
			want:    allowed,
		},
		{
			name:      "a registry section without a mode is allowed",
			request:   moduleConfigRequest("UPDATE"),
			oldObject: moduleConfig("deckhouse", object{}),
			object:    moduleConfig("deckhouse", object{"registry": object{}}),
			want:      allowed,
		},
	}

	skippedBy := make(map[string]bool)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, detail := policy.evaluate(t, test.request, test.oldObject, test.object)
			if got != test.want {
				t.Fatalf("decision = %s (%s), want %s", got, detail, test.want)
			}
			if test.detail != "" && !strings.Contains(detail, test.detail) {
				t.Errorf("detail = %q, want it to contain %q", detail, test.detail)
			}
			if got == skipped {
				skippedBy[detail] = true
			}
		})
	}

	for _, condition := range policy.matchConditions {
		if !skippedBy[condition.name] {
			t.Errorf("no test case is exempted by the %s match condition", condition.name)
		}
	}
}

// The policy used to be rendered only on some clusters: with the registry module disabled, during
// bootstrap or on a cluster that is not static. Proxy and Local are refused everywhere now, so the
// policy has to be there whatever those say.
var _ = Describe("Module :: deckhouse :: helm template :: registry validation policy", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(setup func()) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		setup()
		f.HelmRender(WithAPIVersions(
			"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy",
			"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding",
		))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	assertRendered := func() {
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deckhouse-module-registry-validation")
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.matchConstraints.resourceRules.0.operations").String()).
			To(MatchJSON(`["CREATE", "UPDATE"]`))
		Expect(policy.Field("spec.validations").Array()).To(HaveLen(1))

		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "deckhouse-module-registry-validation")
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
	}

	It("is rendered on a bootstrapped static cluster with the registry module enabled", func() {
		render(func() {
			f.ValuesSet("global.enabledModules", []string{"registry"})
			f.ValuesSet("global.clusterIsBootstrapped", true)
		})
		assertRendered()
	})

	It("is rendered during bootstrap with the registry module disabled", func() {
		render(func() {})
		assertRendered()
	})

	It("is rendered on a cloud cluster", func() {
		render(func() {
			f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
			f.ValuesSetFromYaml("global.clusterConfiguration.cloud", "{prefix: sandbox, provider: AWS}")
			f.ValuesSet("global.enabledModules", []string{"registry"})
			f.ValuesSet("global.clusterIsBootstrapped", true)
		})
		assertRendered()
	})

	It("is not rendered where the API server has no ValidatingAdmissionPolicy", func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deckhouse-module-registry-validation").Exists()).
			To(BeFalse())
	})
})

// -- fixtures ----------------------------------------------------------------

func moduleConfig(name string, settings object) object {
	return object{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   object{"name": name},
		"spec":       object{"version": 1, "settings": settings},
	}
}

// withFinalizers sets the finalizers of a ModuleConfig, as one being deleted has them, and returns it.
func withFinalizers(config object, finalizers ...string) object {
	metadata := config["metadata"].(object)
	metadata["deletionTimestamp"] = "2026-01-01T00:00:00Z"
	metadata["finalizers"] = finalizers
	return config
}

func registrySettings(mode string) object {
	return object{"registry": object{"mode": mode}}
}

func moduleConfigRequest(operation string) object {
	return object{
		"operation": operation,
		"kind":      object{"group": "deckhouse.io", "version": "v1alpha1", "kind": "ModuleConfig"},
		"userInfo": object{
			"username": "admin",
			"groups":   []string{"system:authenticated"},
		},
	}
}
