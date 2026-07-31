/*
Copyright 2025 Flant JSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: create_migration_resources ::", func() {
	const migrationValues = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  nodes: {}
  provider: {}
`

	clusterConfig := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
  replicas: 1
provider:
  cloudID: test-cloud
  folderID: test-folder
  serviceAccountJSON: |-
    {
      "id": "test"
    }
nodeNetworkCIDR: 10.0.0.0/24
sshPublicKey: ssh-rsa AAAA
`)

	pccSecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfig)))

	ns := `
apiVersion: v1
kind: Namespace
metadata:
  name: d8-cloud-provider-yandex
`

	mcV1 := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings: {}
`

	// ---- State A: no PCC — OnAfterHelm does nothing ----
	Context("State A: no PCC — OnAfterHelm does not create migration resources", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(ns)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not create migration resources when PCC is absent", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())
		})
	})

	// ---- State B: PCC present — OnAfterHelm creates migration resources ----
	Context("State B: PCC present — OnAfterHelm creates migration resources", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(pccSecret + "\n---\n" + mcV1 + "\n---\n" + ns)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should create migration resources secret, configmap, ModuleConfig v2, NodeGroup, YandexInstanceClass and d8-credentials", func() {
			Expect(f).To(ExecuteSuccessfully())

			// --- Secret and ConfigMap exist ---
			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())

			// --- Parse resources.yaml ---
			resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
			Expect(resourcesYAML).NotTo(BeEmpty())

			rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
			Expect(err).NotTo(HaveOccurred())

			docs := splitYAMLDocuments(string(rawBytes))

			// --- Verify NodeGroup and YandexInstanceClass ---
			var nodeGroupDoc map[string]any
			var instanceClassDoc map[string]any
			var moduleConfigDoc map[string]any
			var credentialSecretDoc map[string]any

			for _, doc := range docs {
				var obj map[string]any
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					continue
				}

				switch obj["kind"] {
				case "NodeGroup":
					nodeGroupDoc = obj
				case "YandexInstanceClass":
					instanceClassDoc = obj
				case "ModuleConfig":
					moduleConfigDoc = obj
				case "Secret":
					metadata, _ := obj["metadata"].(map[string]any)
					name, _ := metadata["name"].(string)
					if name == "d8-credentials" {
						credentialSecretDoc = obj
					}
				}
			}

			const expectedInstanceClassName = "master-fc613b4dfd67"
			Expect(nodeGroupDoc).NotTo(BeNil(), "NodeGroup document must be present")
			Expect(instanceClassDoc).NotTo(BeNil(), "YandexInstanceClass document must be present")

			instanceClassMetadata, ok := instanceClassDoc["metadata"].(map[string]any)
			Expect(ok).To(BeTrue(), "YandexInstanceClass metadata must be a map")
			Expect(instanceClassMetadata["name"]).To(Equal(expectedInstanceClassName))

			nodeGroupSpec, ok := nodeGroupDoc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec must be a map")
			cloudInstances, ok := nodeGroupSpec["cloudInstances"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec.cloudInstances must be a map")
			classReference, ok := cloudInstances["classReference"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec.cloudInstances.classReference must be a map")
			Expect(classReference["name"]).To(Equal(expectedInstanceClassName))
			Expect(classReference["kind"]).To(Equal("YandexInstanceClass"))

			// --- Verify ModuleConfig v2 ---
			Expect(moduleConfigDoc).NotTo(BeNil(), "ModuleConfig document must be present")

			mcSpec, ok := moduleConfigDoc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec must be a map")
			Expect(mcSpec["enabled"]).To(Equal(true))
			Expect(mcSpec["version"]).To(BeNumerically("==", 2))

			mcSettings, ok := mcSpec["settings"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec.settings must be a map")

			provider, ok := mcSettings["provider"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.provider must be present")
			provParams, ok := provider["parameters"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.provider.parameters must be a map")
			Expect(provParams["cloudID"]).To(Equal("test-cloud"))
			Expect(provParams["folderID"]).To(Equal("test-folder"))

			nodes, ok := mcSettings["nodes"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.nodes must be present")
			_, hasNodesDisabled := nodes["disabled"]
			Expect(hasNodesDisabled).To(BeFalse(), "nodes.disabled must not be explicitly set")

			storage, ok := mcSettings["storage"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.storage must be present")
			_, hasStorageDisabled := storage["disabled"]
			Expect(hasStorageDisabled).To(BeFalse(), "storage.disabled must not be explicitly set")

			nodesParams, ok := nodes["parameters"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.nodes.parameters must be a map")
			Expect(nodesParams["sshPublicKey"]).To(Equal("ssh-rsa AAAA"))
			Expect(nodesParams["layout"]).To(Equal("WithoutNAT"))
			Expect(nodesParams["nodeNetworkCIDR"]).To(Equal("10.0.0.0/24"))

			// --- Verify d8-credentials Secret ---
			Expect(credentialSecretDoc).NotTo(BeNil(), "d8-credentials Secret document must be present")
			Expect(credentialSecretDoc["type"]).To(Equal("cloud-provider.deckhouse.io/credentials"))

			stringData, ok := credentialSecretDoc["stringData"].(map[string]any)
			Expect(ok).To(BeTrue(), "d8-credentials stringData must be a map")
			Expect(stringData["authScheme"]).To(Equal("serviceAccount"))
		})
	})

	// ---- State B: PCC with NAT — migration resources include two credential Secrets ----
	clusterConfigWithNAT := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    imageID: test
    memory: 4096
  replicas: 1
provider:
  cloudID: test-cloud
  folderID: test-folder
  serviceAccountJSON: |-
    {
      "id": "sa-test"
    }
nodeNetworkCIDR: 10.0.0.0/24
sshPublicKey: ssh-rsa AAAACCC
withNATInstance:
  internalSubnetID: subnet-nat
  natInstanceExternalAddress: 84.201.160.150
  exporterAPIKey: nat-exporter-key
`)

	pccSecretWithNAT := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigWithNAT)))

	Context("State B: PCC with NAT — migration resources include d8-credentials and d8-credentials-exporter", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(pccSecretWithNAT + "\n---\n" + mcV1 + "\n---\n" + ns)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should include both d8-credentials and d8-credentials-exporter Secrets", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
			rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
			Expect(err).NotTo(HaveOccurred())

			var credentialSecrets []map[string]any
			for _, doc := range splitYAMLDocuments(string(rawBytes)) {
				var obj map[string]any
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					continue
				}
				if obj["kind"] == "Secret" && obj["type"] == "cloud-provider.deckhouse.io/credentials" {
					credentialSecrets = append(credentialSecrets, obj)
				}
			}

			Expect(credentialSecrets).To(HaveLen(2), "expected exactly 2 credential Secrets in migration resources")

			names := make([]string, len(credentialSecrets))
			for i, s := range credentialSecrets {
				metadata := s["metadata"].(map[string]any)
				names[i] = metadata["name"].(string)
			}
			Expect(names).To(ConsistOf("d8-credentials", "d8-credentials-exporter"))

			// Verify auth schemes
			for _, s := range credentialSecrets {
				metadata := s["metadata"].(map[string]any)
				sd := s["stringData"].(map[string]any)
				switch metadata["name"] {
				case "d8-credentials":
					Expect(sd["authScheme"]).To(Equal("serviceAccount"))
				case "d8-credentials-exporter":
					Expect(sd["authScheme"]).To(Equal("apiToken"))
				}
			}
		})
	})
})

// splitYAMLDocuments splits a multi-document YAML string into individual documents.
func splitYAMLDocuments(multiDoc string) []string {
	var docs []string
	var current string
	for _, line := range splitLines(multiDoc) {
		if line == "---" {
			if current != "" {
				docs = append(docs, current)
			}
			current = ""
		} else {
			if current != "" {
				current += "\n"
			}
			current += line
		}
	}
	if current != "" {
		docs = append(docs, current)
	}
	return docs
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
