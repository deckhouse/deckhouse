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
	"encoding/base64"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A kube-rbac-proxy sidecar verifies its clients with the CA kept in
// global.internal.modules.kubeRBACProxyCA. This module keeps the key pair in d8-system
// (kube-rbac-proxy/secret.yaml), publishes the certificate to the namespaces it creates
// (namespace.yaml) and grants the proxy the reviews it makes (common/rbac/kube-rbac-proxy.yaml).
//
// HelmRender sets both halves of the pair to "test", so these tests cannot tell the certificate from
// the key by value.

var _ = Describe("Module :: deckhouse :: kube-rbac-proxy ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("keeps the CA key pair in d8-system", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "kube-rbac-proxy-ca-key-pair")
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field("data").Map()).To(HaveLen(2))

		for key, value := range map[string]string{
			`tls\.crt`: f.ValuesGet("global.internal.modules.kubeRBACProxyCA.cert").String(),
			`tls\.key`: f.ValuesGet("global.internal.modules.kubeRBACProxyCA.key").String(),
		} {
			decoded, err := base64.StdEncoding.DecodeString(secret.Field("data." + key).String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(decoded)).To(Equal(value), key)
		}
	})

	It("publishes the CA certificate, and nothing else, to every namespace the module creates", func() {
		for _, namespace := range []string{"d8-system", "d8-monitoring", "d8-service-accounts", "d8-cloud-instance-manager"} {
			configMap := f.KubernetesResource("ConfigMap", namespace, "kube-rbac-proxy-ca.crt")
			Expect(configMap.Exists()).To(BeTrue(), "the CA certificate should be published to %s", namespace)
			Expect(configMap.Field("data").Map()).To(HaveLen(1), "a ConfigMap is readable by far more than the key may be")
			Expect(strings.TrimSpace(configMap.Field(`data.ca\.crt`).String())).
				To(Equal(f.ValuesGet("global.internal.modules.kubeRBACProxyCA.cert").String()))
		}
	})

	It("lets the proxy review the tokens of its clients and their access", func() {
		Expect(roleGrants(f.KubernetesGlobalResource("ClusterRole", "d8:rbac-proxy"))).To(Equal(map[string][]string{
			"authentication.k8s.io/tokenreviews":        {"create"},
			"authorization.k8s.io/subjectaccessreviews": {"create"},
		}))
	})
})
