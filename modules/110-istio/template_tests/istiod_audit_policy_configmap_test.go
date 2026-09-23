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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The istiod-service-accounts ConfigMap feeds control-plane-manager's basic audit policy
// (label control-plane-manager.deckhouse.io/extra-audit-policy-config). That policy is
// written to the kube-apiserver extra-file audit-policy.yaml, which is part of the
// kube-apiserver config checksum — so any change of this ConfigMap restarts kube-apiserver
// on every master node. It must therefore be derived from versionMap (constant for a given
// build) and never from versionsToInstall (changes on every canary upgrade step).
var _ = Describe("Module :: istio :: helm template :: istiod-service-accounts ConfigMap", func() {
	f := SetupHelmConfig(``)

	const extraAuditPolicyLabel = "control-plane-manager.deckhouse.io/extra-audit-policy-config"

	// renderBasicAuditPolicy renders the chart with the given versionsToInstall and returns
	// the raw data.basicAuditPolicy of the istiod-service-accounts ConfigMap.
	renderBasicAuditPolicy := func(versionsToInstall string) string {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
		f.ValuesSetFromYaml("istio.internal.versionsToInstall", versionsToInstall)
		f.HelmRender()

		Expect(f.RenderError).ShouldNot(HaveOccurred())

		cm := f.KubernetesResource("ConfigMap", "d8-istio", "istiod-service-accounts")
		Expect(cm.Exists()).To(BeTrue())

		return cm.Field("data.basicAuditPolicy").String()
	}

	serviceAccountsOf := func(basicAuditPolicy string) []string {
		var parsed struct {
			ServiceAccounts []string `json:"serviceAccounts"`
		}
		Expect(yaml.Unmarshal([]byte(basicAuditPolicy), &parsed)).To(Succeed())
		return parsed.ServiceAccounts
	}

	It("should carry the extra-audit-policy label control-plane-manager watches", func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
		f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.25.2"]`)
		f.HelmRender()

		Expect(f.RenderError).ShouldNot(HaveOccurred())

		cm := f.KubernetesResource("ConfigMap", "d8-istio", "istiod-service-accounts")
		Expect(cm.Exists()).To(BeTrue())
		Expect(cm.Field("metadata.labels").Map()).To(HaveKey(extraAuditPolicyLabel))
	})

	It("should list an istiod ServiceAccount for every version in versionMap", func() {
		// Only 1.25.2 is installed, but every supported revision must be listed.
		Expect(serviceAccountsOf(renderBasicAuditPolicy(`["1.25.2"]`))).To(ConsistOf(
			"system:serviceaccount:d8-istio:istiod-v1x25x2",
			"system:serviceaccount:d8-istio:istiod-v1x27x9",
			"system:serviceaccount:d8-istio:istiod-v1x29x6",
		))
	})

	It("should stay byte-identical across every step of a canary upgrade", func() {
		// A canary upgrade walks: single version -> both versions -> promoted version.
		// The ConfigMap must not change, otherwise kube-apiserver is restarted at each step.
		bootstrap := renderBasicAuditPolicy(`["1.25.2"]`)
		canary := renderBasicAuditPolicy(`["1.25.2","1.27.9"]`)
		promoted := renderBasicAuditPolicy(`["1.27.9"]`)
		all := renderBasicAuditPolicy(`["1.25.2","1.27.9","1.29.6"]`)

		Expect(canary).To(Equal(bootstrap), "adding an additionalVersion must not change the audit policy input")
		Expect(promoted).To(Equal(bootstrap), "promoting globalVersion must not change the audit policy input")
		Expect(all).To(Equal(bootstrap), "installing every version must not change the audit policy input")
	})

	It("should not depend on versionsToInstall being set at all", func() {
		withVersions := renderBasicAuditPolicy(`["1.25.2"]`)
		withoutVersions := renderBasicAuditPolicy(`[]`)

		Expect(withoutVersions).To(Equal(withVersions))
	})
})
