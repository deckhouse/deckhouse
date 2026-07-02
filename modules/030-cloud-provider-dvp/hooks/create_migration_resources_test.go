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

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: create_migration_resources ::", func() {
	const (
		migrationValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
  nodes: {}
  provider: {}
`
	)

	kubeconfigDataBase64 := base64.StdEncoding.EncodeToString([]byte("apiVe"))
	clusterConfig := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDisk:
      size: 15Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    rootDisk:
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
      size: 50Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    virtualMachine:
      virtualMachineClassName: superbe-class
      bootloader: EFI
      cpu:
        coreFraction: 100%%
        cores: 4
      memory:
        size: 8Gi
  replicas: 1
provider:
  kubeconfigDataBase64: %s
  namespace: cloud-provider01
sshPublicKey: ssh-rsa AAAAB3N
region: ru-msk-1
zones:
  - default
`, kubeconfigDataBase64)

	pccSecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfig)))

	// ---- State B: PCC present, OnAfterHelm creates migration resources ----
	Context("State B: PCC present — OnAfterHelm creates migration resources secret", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(pccSecret)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should create migration resources secret and configmap", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())
		})

		It("should generate ModuleConfig without explicit disabled fields", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			// Parse the resources.yaml stored in the secret.
			resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
			Expect(resourcesYAML).NotTo(BeEmpty())

			// Decode base64 value (KubeObject.Field returns the raw base64 from data map).
			rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
			Expect(err).NotTo(HaveOccurred())

			// Find the ModuleConfig document within the multi-document YAML.
			var moduleConfigDoc map[string]any
			for _, doc := range splitYAMLDocuments(string(rawBytes)) {
				var obj map[string]any
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					continue
				}
				if obj["kind"] == "ModuleConfig" {
					moduleConfigDoc = obj
					break
				}
			}
			Expect(moduleConfigDoc).NotTo(BeNil(), "ModuleConfig document must be present in resources.yaml")

			spec, ok := moduleConfigDoc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec must be a map")
			settings, ok := spec["settings"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec.settings must be a map")

			// nodes section must NOT contain 'disabled' field — it has a schema default.
			nodes, ok := settings["nodes"].(map[string]any)
			Expect(ok).To(BeTrue(), "nodes must be present in settings")
			_, hasDisabled := nodes["disabled"]
			Expect(hasDisabled).To(BeFalse(), "nodes.disabled must not be explicitly set in generated ModuleConfig")

			// storage section must NOT contain 'disabled' field — it has a schema default.
			storage, ok := settings["storage"].(map[string]any)
			Expect(ok).To(BeTrue(), "storage must be present in settings")
			_, hasStorageDisabled := storage["disabled"]
			Expect(hasStorageDisabled).To(BeFalse(), "storage.disabled must not be explicitly set in generated ModuleConfig")
		})

		It("should generate NodeGroup and DVPInstanceClass with hashed instance class name", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
			Expect(resourcesYAML).NotTo(BeEmpty())

			rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
			Expect(err).NotTo(HaveOccurred())

			var nodeGroupDoc map[string]any
			var instanceClassDoc map[string]any
			for _, doc := range splitYAMLDocuments(string(rawBytes)) {
				var obj map[string]any
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					continue
				}

				switch obj["kind"] {
				case "NodeGroup":
					nodeGroupDoc = obj
				case "DVPInstanceClass":
					instanceClassDoc = obj
				}
			}

			const expectedInstanceClassName = "master-fc613b4dfd67"
			Expect(nodeGroupDoc).NotTo(BeNil(), "NodeGroup document must be present in resources.yaml")
			Expect(instanceClassDoc).NotTo(BeNil(), "DVPInstanceClass document must be present in resources.yaml")

			instanceClassMetadata, ok := instanceClassDoc["metadata"].(map[string]any)
			Expect(ok).To(BeTrue(), "DVPInstanceClass metadata must be a map")
			Expect(instanceClassMetadata["name"]).To(Equal(expectedInstanceClassName))

			nodeGroupSpec, ok := nodeGroupDoc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec must be a map")
			cloudInstances, ok := nodeGroupSpec["cloudInstances"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec.cloudInstances must be a map")
			classReference, ok := cloudInstances["classReference"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec.cloudInstances.classReference must be a map")
			Expect(classReference["name"]).To(Equal(expectedInstanceClassName))

			// Zones from the source PCC ("default") must be preserved through the
			// migration pipeline — see State B (no zones) for the empty case.
			Expect(cloudInstances["zones"]).To(Equal([]any{"default"}),
				"zones from the source PCC must be preserved in the migrated NodeGroup")
		})
	})

	clusterConfigNoZones := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDisk:
      size: 15Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    rootDisk:
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
      size: 50Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    virtualMachine:
      virtualMachineClassName: superbe-class
      bootloader: EFI
      cpu:
        coreFraction: 100%%
        cores: 4
      memory:
        size: 8Gi
  replicas: 1
provider:
  kubeconfigDataBase64: %s
  namespace: cloud-provider01
sshPublicKey: ssh-rsa AAAAB3N
region: ru-msk-1
`, kubeconfigDataBase64)

	pccSecretNoZones := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigNoZones)))

	// ---- State B (no zones): PCC without zones — fallback must NOT add "default" ----
	Context("State B (no zones): PCC without zones — NodeGroup must have empty zones", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(pccSecretNoZones)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not add default zone to NodeGroup when PCC has no zones", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
			Expect(resourcesYAML).NotTo(BeEmpty())

			rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
			Expect(err).NotTo(HaveOccurred())

			var nodeGroupDoc map[string]any
			for _, doc := range splitYAMLDocuments(string(rawBytes)) {
				var obj map[string]any
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					continue
				}
				if obj["kind"] == "NodeGroup" {
					nodeGroupDoc = obj
					break
				}
			}
			Expect(nodeGroupDoc).NotTo(BeNil(), "NodeGroup document must be present in resources.yaml")

			spec, ok := nodeGroupDoc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec must be a map")
			cloudInstances, ok := spec["cloudInstances"].(map[string]any)
			Expect(ok).To(BeTrue(), "NodeGroup spec.cloudInstances must be a map")

			// zones must NOT contain the synthetic "default" when the source PCC had
			// no zones. The migration path returns nil so the rendered NodeGroup has
			// zones: null (or the key is absent), letting node-manager apply its own
			// fallback rather than forcing "default".
			switch zones := cloudInstances["zones"].(type) {
			case []any:
				Expect(zones).To(BeEmpty(), "zones must be empty when PCC has no zones, not [default]")
			case nil:
				// zones: null or absent — acceptable, no synthetic default injected.
			default:
				Fail(fmt.Sprintf("unexpected zones type %T: %#v", zones, zones))
			}
		})
	})

	// ---- State A: no PCC — OnAfterHelm does nothing ----
	Context("State A: no PCC — OnAfterHelm does not create migration resources", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not create migration resources when PCC is absent", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())
		})
	})

	// migrationResourceDocuments decodes the resources.yaml bundle and indexes documents by kind.
	migrationResourceDocuments := func(f *HookExecutionConfig) map[string][]map[string]any {
		migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
		resourcesYAML := migrationSecret.Field("data.resources\\.yaml").String()
		Expect(resourcesYAML).NotTo(BeEmpty())

		rawBytes, err := base64.StdEncoding.DecodeString(resourcesYAML)
		Expect(err).NotTo(HaveOccurred())

		byKind := map[string][]map[string]any{}
		for _, doc := range splitYAMLDocuments(string(rawBytes)) {
			var obj map[string]any
			if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
				continue
			}
			kind, _ := obj["kind"].(string)
			byKind[kind] = append(byKind[kind], obj)
		}
		return byKind
	}

	Context("Hybrid migration: no PCC and legacy ModuleConfig v1 has kubeconfig", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
		BeforeEach(func() {
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  enabled: true
  settings:
    provider:
      kubeconfigDataBase64: %s
      namespace: cloud-provider01
    zones:
      - zone-a
      - zone-b
`, kubeconfigDataBase64))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should create migration resources with d8-credentials Secret from legacy ModuleConfig kubeconfig", func() {
			Expect(f).To(ExecuteSuccessfully())

			credentialsSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-credentials")
			Expect(credentialsSecret.Exists()).To(BeFalse())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())

			docs := migrationResourceDocuments(f)

			credentialsSecretDoc := docs["Secret"]
			Expect(credentialsSecretDoc).To(HaveLen(1), "exactly one Secret document expected in resources.yaml")
			Expect(credentialsSecretDoc[0]["type"]).To(Equal("cloud-provider.deckhouse.io/credentials"))

			stringData, ok := credentialsSecretDoc[0]["stringData"].(map[string]any)
			Expect(ok).To(BeTrue(), "d8-credentials stringData must be a map")
			Expect(stringData["authScheme"]).To(Equal("kubeconfig"))
			Expect(stringData["secret"]).To(Equal(kubeconfigDataBase64))
		})

		It("should include a ModuleConfig v2 mirroring the v1->v2 conversion and no node resources", func() {
			Expect(f).To(ExecuteSuccessfully())

			docs := migrationResourceDocuments(f)

			Expect(docs["NodeGroup"]).To(BeEmpty(), "hybrid migration must not generate NodeGroups")
			Expect(docs["DVPInstanceClass"]).To(BeEmpty(), "hybrid migration must not generate DVPInstanceClasses")

			mcDocs := docs["ModuleConfig"]
			Expect(mcDocs).To(HaveLen(1), "hybrid migration must include exactly one ModuleConfig v2")
			mc := mcDocs[0]

			spec, ok := mc["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec must be a map")
			Expect(spec["enabled"]).To(Equal(true))
			Expect(spec["version"]).To(BeNumerically("==", 2))

			settings, ok := spec["settings"].(map[string]any)
			Expect(ok).To(BeTrue(), "ModuleConfig spec.settings must be a map")

			provider, ok := settings["provider"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.provider must be a map")
			providerParams, ok := provider["parameters"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.provider.parameters must be a map")
			Expect(providerParams["namespace"]).To(Equal("cloud-provider01"))
			_, hasKubeconfig := provider["kubeconfigDataBase64"]
			Expect(hasKubeconfig).To(BeFalse(), "kubeconfigDataBase64 must not be present in ModuleConfig v2")

			nodes, ok := settings["nodes"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.nodes must be a map")
			_, hasNodesDisabled := nodes["disabled"]
			Expect(hasNodesDisabled).To(BeFalse(), "nodes.disabled must not be explicitly set (schema default)")
			nodesParams, ok := nodes["parameters"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.nodes.parameters must be a map")
			Expect(nodesParams["layout"]).To(Equal("Standard"))
			Expect(nodesParams["sshPublicKey"]).To(Equal("ssh-rsa PLACEHOLDER_REPLACE_ME"))
			Expect(nodesParams["zones"]).To(Equal([]any{"zone-a", "zone-b"}),
				"legacy top-level zones must move into nodes.parameters.zones")

			storage, ok := settings["storage"].(map[string]any)
			Expect(ok).To(BeTrue(), "settings.storage must be a map")
			_, hasStorageDisabled := storage["disabled"]
			Expect(hasStorageDisabled).To(BeFalse(), "storage.disabled must not be explicitly set (schema default)")
		})
	})

	Context("Hybrid migration: no PCC and legacy ModuleConfig v1 without namespace or zones", func() {
		f := HookExecutionConfigInit(migrationValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
		BeforeEach(func() {
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  enabled: true
  settings:
    provider:
      kubeconfigDataBase64: %s
`, kubeconfigDataBase64))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should default namespace to 'default' and omit zones", func() {
			Expect(f).To(ExecuteSuccessfully())

			docs := migrationResourceDocuments(f)
			mcDocs := docs["ModuleConfig"]
			Expect(mcDocs).To(HaveLen(1))

			settings := mcDocs[0]["spec"].(map[string]any)["settings"].(map[string]any)
			providerParams := settings["provider"].(map[string]any)["parameters"].(map[string]any)
			Expect(providerParams["namespace"]).To(Equal("default"))

			nodesParams := settings["nodes"].(map[string]any)["parameters"].(map[string]any)
			_, hasZones := nodesParams["zones"]
			Expect(hasZones).To(BeFalse(), "zones must be absent when the legacy ModuleConfig has none")
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
