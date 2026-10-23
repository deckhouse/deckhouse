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
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// dexStorageRule returns the verbs a role grants on resource of the Dex storage group, or nil.
func dexStorageRule(role object_store.KubeObject, resource string) []string {
	for _, rule := range role.Field("rules").Array() {
		if !containsString(rule.Get("apiGroups"), "dex.coreos.com") || !containsString(rule.Get("resources"), resource) {
			continue
		}
		verbs := []string{}
		for _, verb := range rule.Get("verbs").Array() {
			verbs = append(verbs, verb.String())
		}
		return verbs
	}
	return nil
}

func containsString(list gjson.Result, value string) bool {
	for _, item := range list.Array() {
		if item.String() == value {
			return true
		}
	}
	return false
}

var _ = Describe("Module :: user-authn :: helm template :: Dex storage RBAC", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.HelmRender()
	})

	// Dex v2.46 keeps second factors, locks, and browser sessions in UserIdentity and
	// AuthSession. Dex lists them at start and the admin kubeconfig manages the Dex storage as a
	// whole: without the rights Dex fails to start. The controller copies the older places into
	// UserIdentity; it does not touch AuthSession yet.
	It("Should grant the Dex v2.46 storage types wherever the Dex storage is granted", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		roles := map[string]object_store.KubeObject{
			"dex ClusterRole":                     hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:dex:crd"),
			"dex Role":                            hec.KubernetesResource("Role", "d8-user-authn", "dex"),
			"d8:user-authn:admin-kubeconfig role": hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:admin-kubeconfig"),
		}
		for name, role := range roles {
			Expect(role.Exists()).To(BeTrue(), name)
			for _, resource := range []string{"useridentities", "authsessions"} {
				Expect(dexStorageRule(role, resource)).ToNot(BeEmpty(), "%s grants nothing on %s", name, resource)
			}
		}

		dexVerbs := dexStorageRule(roles["dex ClusterRole"], "useridentities")
		Expect(dexVerbs).To(ContainElements("get", "list", "watch", "create", "update", "delete"))

		// The controller writes UserIdentity with Create and Update only.
		controllerRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:controller")
		Expect(dexStorageRule(controllerRole, "useridentities")).To(ConsistOf("get", "list", "watch", "create", "update", "delete"))
		Expect(dexStorageRule(controllerRole, "authsessions")).To(BeEmpty())
	})
})
