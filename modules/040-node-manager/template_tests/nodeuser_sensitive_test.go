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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

const nodeUserCRDPath = "../crds/nodeuser.yaml"

var nodeUserServedVersions = []string{"v1alpha1", "v1"}

var _ = Describe("Module :: node-manager :: NodeUser CRD :: sensitive fields ::", func() {
	specProperties := func() map[string]map[string]interface{} {
		raw, err := os.ReadFile(nodeUserCRDPath)
		Expect(err).ShouldNot(HaveOccurred())

		// Deliberately untyped: x-kubernetes-sensitive-data is a Deckhouse
		// extension and is not present in the upstream CRD Go types.
		var crd map[string]interface{}
		Expect(yaml.Unmarshal(raw, &crd)).To(Succeed())

		spec, ok := crd["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "spec is missing in %s", nodeUserCRDPath)
		versions, ok := spec["versions"].([]interface{})
		Expect(ok).To(BeTrue(), "spec.versions is missing in %s", nodeUserCRDPath)

		byVersion := map[string]map[string]interface{}{}
		for _, item := range versions {
			version, ok := item.(map[string]interface{})
			Expect(ok).To(BeTrue())
			if served, ok := version["served"].(bool); ok && !served {
				continue
			}

			name, ok := version["name"].(string)
			Expect(ok).To(BeTrue(), "version without a name in %s", nodeUserCRDPath)
			properties, ok := version["schema"].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})["properties"].(map[string]interface{})["spec"].(map[string]interface{})["properties"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "cannot reach spec.properties of version %s", name)
			byVersion[name] = properties
		}
		return byVersion
	}

	It("marks passwordHash in every served version", func() {
		properties := specProperties()
		Expect(properties).To(HaveLen(len(nodeUserServedVersions)))

		for _, version := range nodeUserServedVersions {
			passwordHash, ok := properties[version]["passwordHash"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "%s: spec.passwordHash is missing", version)
			Expect(passwordHash["x-kubernetes-sensitive-data"]).To(BeTrue(),
				"%s: spec.passwordHash must not be returned without nodeusers/sensitive access", version)
		}
	})

	It("keeps non-secret fields readable", func() {
		properties := specProperties()
		for _, version := range nodeUserServedVersions {
			for _, field := range []string{"uid", "sshPublicKey", "sshPublicKeys", "isSudoer"} {
				property, ok := properties[version][field].(map[string]interface{})
				Expect(ok).To(BeTrue(), "%s: spec.%s is missing", version, field)
				Expect(property).ToNot(HaveKey("x-kubernetes-sensitive-data"),
					"%s: spec.%s must stay visible", version, field)
			}
		}
	})
})
