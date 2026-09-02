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
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Module :: node-manager :: helm template :: NodeConfigTemplate rights", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("hands the template to whoever may add a node, and to nobody else", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		edit := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:edit")
		view := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:view")
		Expect(edit.Exists()).To(BeTrue())
		Expect(view.Exists()).To(BeTrue())

		Expect(nodeConfigTemplateVerbs(edit)).To(ConsistOf("get", "list"))
		// A rendered template carries a live bootstrap token: a read-only role
		// that could fetch one could join machines to the cluster.
		Expect(nodeConfigTemplateVerbs(view)).To(BeEmpty())
	})
})

func nodeConfigTemplateVerbs(role object_store.KubeObject) []string {
	var rules []rbacv1.PolicyRule
	Expect(json.Unmarshal([]byte(role.Field("rules").String()), &rules)).To(Succeed())

	for _, rule := range rules {
		if !slices.Contains(rule.APIGroups, "templates.internal.deckhouse.io") {
			continue
		}
		if slices.Contains(rule.Resources, "nodeconfigtemplates") {
			return rule.Verbs
		}
	}
	return nil
}

// kube-aggregator routes by group version: an APIService that names a service
// takes the whole group version to that service, and the CRDs of the same group
// version stop being served. NodeConfig is a CRD in internal.deckhouse.io, and
// every Deckhouse Engine node reads it, so the aggregated template must live elsewhere.
var _ = Describe("Module :: node-manager :: helm template :: aggregated API group", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("does not aggregate the group the NodeConfig CRD is served from", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		api := f.KubernetesGlobalResource("APIService", "v1alpha1.templates.internal.deckhouse.io")
		Expect(api.Exists()).To(BeTrue())
		Expect(api.Field("spec.group").String()).To(Equal("templates.internal.deckhouse.io"))
		Expect(api.Field("spec.service.name").String()).To(Equal("node-controller-webhook"))

		Expect(f.KubernetesGlobalResource("APIService", "v1alpha1.internal.deckhouse.io").Exists()).To(BeFalse(),
			"an APIService here would take nodeconfigs away from the CRD that serves them")
	})
})
