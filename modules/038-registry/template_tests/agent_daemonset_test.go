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

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const agentModuleValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  cache:
    enabled: false
  pki:
    hash: "h"
    httpSecret: HTTP_SECRET
    ca: {cert: CA_CERT, key: CA_KEY}
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
`

var _ = Describe("Module :: registry :: helm template :: registry-agent", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", agentModuleValues)
		f.HelmRender()
	})

	It("renders DaemonSet registry-agent with hostNetwork and containerd mount", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		ds := f.KubernetesResource("DaemonSet", "d8-system", "registry-agent")
		Expect(ds.Exists()).To(BeTrue())

		// hostNetwork must be true
		Expect(ds.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())

		// must mount /etc/containerd/registry.d
		volumeMounts := ds.Field("spec.template.spec.containers.0.volumeMounts").Array()
		var foundMount bool
		for _, vm := range volumeMounts {
			if vm.Get("mountPath").String() == "/etc/containerd/registry.d" {
				foundMount = true
				break
			}
		}
		Expect(foundMount).To(BeTrue(), "expected volumeMount at /etc/containerd/registry.d")

		// hostPath volume for registry.d must exist
		volumes := ds.Field("spec.template.spec.volumes").Array()
		var foundVolume bool
		for _, v := range volumes {
			if v.Get("hostPath.path").String() == "/etc/containerd/registry.d" {
				foundVolume = true
				break
			}
		}
		Expect(foundVolume).To(BeTrue(), "expected hostPath volume for /etc/containerd/registry.d")

		// readinessProbe must target /readyz on 127.0.0.1:5051
		Expect(ds.Field("spec.template.spec.containers.0.readinessProbe.httpGet.path").String()).To(Equal("/readyz"))
		Expect(ds.Field("spec.template.spec.containers.0.readinessProbe.httpGet.port").Int()).To(Equal(int64(5051)))
		Expect(ds.Field("spec.template.spec.containers.0.readinessProbe.httpGet.host").String()).To(Equal("127.0.0.1"))

		// registry-bootstrap volumeMount must exist at /etc/kubernetes/registry-agent-bootstrap
		var foundBootstrapMount bool
		for _, vm := range volumeMounts {
			if vm.Get("mountPath").String() == "/etc/kubernetes/registry-agent-bootstrap" {
				foundBootstrapMount = true
				break
			}
		}
		Expect(foundBootstrapMount).To(BeTrue(), "expected volumeMount at /etc/kubernetes/registry-agent-bootstrap")

		// registry-bootstrap volume must exist with optional: true
		var foundBootstrapVolume bool
		for _, v := range volumes {
			if v.Get("name").String() == "registry-bootstrap" {
				Expect(v.Get("secret.optional").Bool()).To(BeTrue(), "registry-bootstrap volume must have optional: true")
				foundBootstrapVolume = true
				break
			}
		}
		Expect(foundBootstrapVolume).To(BeTrue(), "expected registry-bootstrap secret volume")
	})

	It("renders ServiceAccount registry-agent in d8-system", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		sa := f.KubernetesResource("ServiceAccount", "d8-system", "registry-agent")
		Expect(sa.Exists()).To(BeTrue())
	})

	It("renders ClusterRole d8:registry:registry-agent with registryconfigs permissions", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		cr := f.KubernetesGlobalResource("ClusterRole", "d8:registry:registry-agent")
		Expect(cr.Exists()).To(BeTrue())

		rules := cr.Field("rules").Array()
		Expect(len(rules)).To(BeNumerically(">=", 2))
	})
})
