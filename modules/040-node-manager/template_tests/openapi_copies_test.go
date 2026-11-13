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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

// The EE and CSE copies of the config-values replace the CE files in full.
// The copies must stay equal to CE, except for the fencing settings.
var _ = Describe("Module :: node-manager :: openapi copies of config-values", func() {
	readWithoutFencing := func(path string) map[string]interface{} {
		data, err := os.ReadFile(path)
		Expect(err).ShouldNot(HaveOccurred())

		var doc map[string]interface{}
		Expect(yaml.Unmarshal(data, &doc)).Should(Succeed())

		properties, ok := doc["properties"].(map[string]interface{})
		Expect(ok).Should(BeTrue(), path)
		delete(properties, "fencing")

		return doc
	}

	DescribeTable("is equal to CE without the fencing settings",
		func(file, copyPath string) {
			ce := readWithoutFencing("../openapi/" + file)
			Expect(readWithoutFencing(copyPath)).Should(Equal(ce))
		},
		Entry("EE config-values", "config-values.yaml", "../../../ee/modules/040-node-manager/openapi/config-values.yaml"),
		Entry("CSE config-values", "config-values.yaml", "../../../ee/cse/modules/040-node-manager/openapi/config-values.yaml"),
		Entry("EE doc-ru-config-values", "doc-ru-config-values.yaml", "../../../ee/modules/040-node-manager/openapi/doc-ru-config-values.yaml"),
		Entry("CSE doc-ru-config-values", "doc-ru-config-values.yaml", "../../../ee/cse/modules/040-node-manager/openapi/doc-ru-config-values.yaml"),
	)
})
