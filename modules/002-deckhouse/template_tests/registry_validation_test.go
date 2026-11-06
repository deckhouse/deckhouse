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
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/cel-go/cel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// registry/validation.yaml guards the registry settings in the deckhouse ModuleConfig while changing
// them is not safe: while the registry module is disabled, until the cluster is bootstrapped, on a
// cluster that is not static, and during the Local and Proxy modes of the registry module. The policy
// exists only while one of these holds, and carries a validation for each one that does.
//
// The validations are evaluated in the base CEL environment of kube-apiserver at the oldest Kubernetes
// version the release supports: with failurePolicy Fail, an expression that does not compile there
// would deny every change of the deckhouse ModuleConfig.

const registryValidationPolicy = "deckhouse-module-registry-validation"

// oldestKubernetesVersion is the lowest Kubernetes version of candi/version_map.yml, the oldest
// cluster a release from this branch runs on.
func oldestKubernetesVersion() *version.Version {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "candi", "version_map.yml"))
	Expect(err).ShouldNot(HaveOccurred())

	var versionMap struct {
		K8s map[string]json.RawMessage `json:"k8s"`
	}
	Expect(yaml.Unmarshal(raw, &versionMap)).To(Succeed())
	Expect(versionMap.K8s).ToNot(BeEmpty())

	var oldest *version.Version
	for supported := range versionMap.K8s {
		parsed, err := version.ParseGeneric(supported)
		Expect(err).ShouldNot(HaveOccurred(), "Kubernetes version %q", supported)
		if oldest == nil || parsed.LessThan(oldest) {
			oldest = parsed
		}
	}
	return oldest
}

// admissionPolicyVerdict is what kube-apiserver does with a request to a rendered
// ValidatingAdmissionPolicy: a request a match condition rejects is not validated, and a validated one
// is denied with the message of the first validation that does not hold. An expression that fails to
// evaluate fails the test, where kube-apiserver would apply the failure policy of the policy to it.
func admissionPolicyVerdict(policy object_store.KubeObject, request, oldObject, newObject map[string]any) (verdict, string) {
	Expect(policy.Exists()).To(BeTrue(), "the policy should be rendered")

	env, err := environment.MustBaseEnvSet(oldestKubernetesVersion()).Env(environment.NewExpressions)
	Expect(err).ShouldNot(HaveOccurred())
	env, err = env.Extend(
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
	)
	Expect(err).ShouldNot(HaveOccurred())

	activation := map[string]any{"request": request, "object": celValue(newObject), "oldObject": celValue(oldObject)}
	holds := func(expression string) bool {
		ast, issues := env.Compile(expression)
		Expect(issues.Err()).ShouldNot(HaveOccurred(), "the expression should compile: %s", expression)
		Expect(ast.OutputType()).To(Equal(cel.BoolType), "the expression should be a condition: %s", expression)
		program, err := env.Program(ast)
		Expect(err).ShouldNot(HaveOccurred())
		out, _, err := program.Eval(activation)
		Expect(err).ShouldNot(HaveOccurred(), "the expression should evaluate: %s", expression)
		return out.Value() == true
	}

	for _, condition := range policy.Field("spec.matchConditions").Array() {
		if !holds(condition.Get("expression").String()) {
			return verdictSkipped, ""
		}
	}
	for _, validation := range policy.Field("spec.validations").Array() {
		if !holds(validation.Get("expression").String()) {
			return verdictDenied, validation.Get("message").String()
		}
	}
	return verdictAllowed, ""
}

// deckhouseModuleConfig is a ModuleConfig named deckhouse with the given registry settings, or none
// when they are nil.
func deckhouseModuleConfig(registry map[string]any) map[string]any {
	settings := map[string]any{"releaseChannel": "Stable"}
	if registry != nil {
		settings["registry"] = registry
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]any{"name": "deckhouse"},
		"spec":       map[string]any{"version": int64(1), "settings": settings},
	}
}

var _ = Describe("Module :: deckhouse :: registry settings policy ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	// A cluster where nothing restricts the registry settings: the registry module is enabled, the
	// cluster is bootstrapped and static, and the registry module runs in the Direct mode.
	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "registry"]`)
		f.ValuesSet("global.clusterIsBootstrapped", true)
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.ValuesSet("deckhouse.internal.registry.mode", "Direct")
	})

	render := func() {
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	policy := func() object_store.KubeObject {
		return f.KubernetesGlobalResource("ValidatingAdmissionPolicy", registryValidationPolicy)
	}

	// update is the verdict on an update of the deckhouse ModuleConfig from the old registry settings
	// to the new ones, nil standing for no settings.
	update := func(oldRegistry, newRegistry map[string]any) verdict {
		request := map[string]any{"operation": "UPDATE", "userInfo": map[string]any{"username": "admin@example.com"}}
		decision, _ := admissionPolicyVerdict(policy(), request, deckhouseModuleConfig(oldRegistry), deckhouseModuleConfig(newRegistry))
		return decision
	}

	direct := map[string]any{"mode": "Direct", "direct": map[string]any{"imagesRepo": "registry.example.com/deckhouse"}}
	unmanaged := map[string]any{"mode": "Unmanaged", "unmanaged": map[string]any{"imagesRepo": "registry.example.com/deckhouse"}}

	It("is not rendered while nothing restricts the registry settings", func() {
		render()

		Expect(policy().Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", registryValidationPolicy).Exists()).To(BeFalse())
	})

	It("is not rendered where the API server serves no ValidatingAdmissionPolicy", func() {
		f.ValuesSet("global.clusterIsBootstrapped", false)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(policy().Exists()).To(BeFalse())
	})

	Context("while the registry module is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]`)
			render()
		})

		It("reviews the updates of the deckhouse ModuleConfig only, and denies what it refuses", func() {
			Expect(policy().Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy().Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(`[
				{"apiGroups":["deckhouse.io"],"apiVersions":["v1alpha1"],"operations":["UPDATE"],"resources":["moduleconfigs"]}
			]`))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", registryValidationPolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(registryValidationPolicy))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))

			other := deckhouseModuleConfig(nil)
			other["metadata"] = map[string]any{"name": "registry"}
			request := map[string]any{"operation": "UPDATE"}
			decision, _ := admissionPolicyVerdict(policy(), request, other, other)
			Expect(decision).To(Equal(verdictSkipped), "the other ModuleConfigs are not reviewed")
		})

		It("keeps the registry settings as they are", func() {
			Expect(policy().Field("spec.validations").Array()).To(HaveLen(1))
			Expect(update(nil, direct)).To(Equal(verdictDenied), "adding the settings")
			Expect(update(direct, unmanaged)).To(Equal(verdictDenied), "changing the settings")
			Expect(update(direct, nil)).To(Equal(verdictDenied), "removing the settings")
			Expect(update(direct, direct)).To(Equal(verdictAllowed), "the settings stay as they are")
			Expect(update(nil, nil)).To(Equal(verdictAllowed), "there are no settings")

			_, message := admissionPolicyVerdict(policy(), map[string]any{"operation": "UPDATE"}, deckhouseModuleConfig(nil), deckhouseModuleConfig(direct))
			Expect(message).To(ContainSubstring("when the registry module is disabled"))
		})
	})

	It("keeps the registry settings as they are until the cluster is bootstrapped", func() {
		f.ValuesSet("global.clusterIsBootstrapped", false)
		render()

		Expect(policy().Field("spec.validations").Array()).To(HaveLen(1))
		Expect(update(nil, direct)).To(Equal(verdictDenied))
		Expect(update(direct, unmanaged)).To(Equal(verdictDenied))
		Expect(update(direct, direct)).To(Equal(verdictAllowed))

		_, message := admissionPolicyVerdict(policy(), map[string]any{"operation": "UPDATE"}, deckhouseModuleConfig(direct), deckhouseModuleConfig(unmanaged))
		Expect(message).To(ContainSubstring("during cluster bootstrap"))
	})

	It("refuses the Proxy and Local modes on a cluster that is not static", func() {
		f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
		f.ValuesSetFromYaml("global.clusterConfiguration.cloud", `{provider: OpenStack}`)
		render()

		Expect(policy().Field("spec.validations").Array()).To(HaveLen(1))
		Expect(update(direct, map[string]any{"mode": "Proxy", "proxy": map[string]any{}})).To(Equal(verdictDenied))
		Expect(update(direct, map[string]any{"mode": "Local"})).To(Equal(verdictDenied))
		Expect(update(direct, unmanaged)).To(Equal(verdictAllowed))
		Expect(update(unmanaged, direct)).To(Equal(verdictAllowed))
	})

	It("lets the Local mode be left only for the Direct mode or for an Unmanaged mode with settings of its own", func() {
		f.ValuesSet("deckhouse.internal.registry.mode", "Local")
		render()

		local := map[string]any{"mode": "Local"}
		Expect(policy().Field("spec.validations").Array()).To(HaveLen(1))
		Expect(update(local, map[string]any{"mode": "Proxy", "proxy": map[string]any{}})).To(Equal(verdictDenied), "the Proxy mode")
		Expect(update(local, map[string]any{"mode": "Unmanaged"})).To(Equal(verdictDenied), "an Unmanaged mode without settings")
		Expect(update(local, nil)).To(Equal(verdictDenied), "no registry settings")
		Expect(update(local, map[string]any{})).To(Equal(verdictDenied), "empty registry settings")
		Expect(update(local, direct)).To(Equal(verdictAllowed))
		Expect(update(local, unmanaged)).To(Equal(verdictAllowed))
		Expect(update(local, local)).To(Equal(verdictAllowed))
	})

	It("refuses to leave the Proxy mode for the Local mode", func() {
		f.ValuesSet("deckhouse.internal.registry.mode", "Proxy")
		render()

		proxy := map[string]any{"mode": "Proxy", "proxy": map[string]any{"imagesRepo": "registry.example.com/deckhouse"}}
		Expect(policy().Field("spec.validations").Array()).To(HaveLen(1))
		Expect(update(proxy, map[string]any{"mode": "Local"})).To(Equal(verdictDenied))
		Expect(update(proxy, direct)).To(Equal(verdictAllowed))
		Expect(update(proxy, unmanaged)).To(Equal(verdictAllowed))
		Expect(update(proxy, proxy)).To(Equal(verdictAllowed))
	})

	It("carries a validation for every restriction that holds", func() {
		f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]`)
		f.ValuesSet("global.clusterIsBootstrapped", false)
		f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
		f.ValuesSetFromYaml("global.clusterConfiguration.cloud", `{provider: OpenStack}`)
		f.ValuesSet("deckhouse.internal.registry.mode", "Proxy")
		render()

		Expect(policy().Field("spec.validations").Array()).To(HaveLen(4))
	})
})
