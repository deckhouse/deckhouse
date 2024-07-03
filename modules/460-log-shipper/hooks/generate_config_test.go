/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"encoding/base64"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const namespaceManifest = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-log-shipper
---
`

var _ = Describe("Log shipper :: generate config from crd ::", func() {
	f := HookExecutionConfigInit(`{"logShipper": {"internal": {"activated": false}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLoggingConfig", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLogDestination", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "PodLoggingConfig", true)

	Context("With no namespace", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Should not create the configmap", func() {
			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(BeEmpty())
		})
	})

	Context("File to Non-existed destination", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(namespaceManifest + `
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: File
  file:
    include: ["/var/log/kube-audit/audit.log"]
  destinationRefs:
    - non-existed
`))
			f.RunHook()
		})

		It("Should ignore the pipline for the secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(BeEmpty())
		})
	})

	DescribeTable("React to Custom Resources",
		func(folder string) {
			folder = filepath.Join("testdata", folder)

			manifests, err := os.ReadFile(filepath.Join(folder, "manifests.yaml"))
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.KubeStateSet(namespaceManifest + string(manifests)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)

			goldenFileData, err := os.ReadFile(filepath.Join(folder, "result.json"))
			Expect(err).To(BeNil())

			// Automatically save generated configs to golden files.
			// Use it only if you are aware of changes that caused a diff between generated configs and golden files.
			if os.Getenv("D8_LOG_SHIPPER_SAVE_TESTDATA") == "yes" {
				err := os.WriteFile(filepath.Join(folder, "result.json"), d, 0600)
				Expect(err).To(BeNil())
			}

			assert.JSONEq(GinkgoT(), string(goldenFileData), string(d))

			f.BindingContexts.Set(f.KubeStateSet(namespaceManifest))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
		},
		Entry("Simple pair", "simple-pair"),
		Entry("One source with multiple dests", "multiple-dest"),
		Entry("Multinamespace source with one destination", "one-dest"),
		Entry("Namespaced source", "namespaced-source"),
		Entry("Namespaced with multiline", "multiline"),
		Entry("Namespaced with multiline custom parser", "multiline-custom-pods"),
		Entry("Multiline custom parser", "multiline-custom"),
		Entry("Simple pair with datastream", "pair-datastream"),
		Entry("Simple pair for ES 5.X", "es-5x"),
		Entry("Throttle Transform", "throttle"),
		Entry("File to Elasticsearch", "file-to-elastic"),
		Entry("File to Vector", "file-to-vector"),
		Entry("File to Kafka", "file-to-kafka"),
		Entry("File to Kafka with client certificate authentication", "file-to-kafka-tls"),
		Entry("File to Loki", "file-to-loki"),
		Entry("File to Socket", "file-to-socket"),
		Entry("File to Splunk", "file-to-splunk"),
		Entry("Two sources to single destination", "many-to-one"),
		Entry("Throttle Transform with filter", "throttle-with-filter"),
	)
})
