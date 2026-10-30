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
)

// spec.path of a LocalPathProvisioner is a free-form string from a cluster resource. It is
// rendered into the JSON config of the provisioner. A value that closes the JSON string, breaks the line and adds a
// "---" separator must stay inside its scalar instead of adding documents to the module release.
// YAML also takes NEL (U+0085), which JSON leaves unescaped, for a line break.
var _ = Describe("Module :: local-path-provisioner :: helm template :: yaml injection", func() {
	f := SetupHelmConfig(``)

	for _, tc := range []struct{ name, payload string }{
		{"newline", "/opt/pwned\"]}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pwned\n  namespace: d8-local-path-provisioner"},
		{"NEL", "/opt/pwned\u0085kind:Secret"},
	} {
		payload := tc.payload
		Context("Payload with "+tc.name+" in spec.path", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSet("localPathProvisioner.internal.localPathProvisioners", []interface{}{
					map[string]interface{}{"name": "evil", "spec": map[string]interface{}{"path": payload}},
				})
				f.HelmRender()
			})

			It("Must render and must not produce injected documents", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "pwned").Exists()).To(BeFalse())

				cm := f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-evil")
				Expect(cm.Exists()).To(BeTrue())
				config := cm.Field("data.config\\.json").String()
				Expect(gjson.Valid(config)).To(BeTrue(), config)
				Expect(gjson.Get(config, "nodePathMap.0.paths").Array()).To(HaveLen(1))
				Expect(gjson.Get(config, "nodePathMap.0.paths.0").String()).To(Equal(payload))
				Expect(cm.Field("data.helperPod\\.yaml").String()).To(HavePrefix("apiVersion: v1\nkind: Pod\n"))
			})
		})
	}
})
