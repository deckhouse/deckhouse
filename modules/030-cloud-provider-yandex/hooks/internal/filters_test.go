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

package internal

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	validDiscoveryData = `{
  "apiVersion": "deckhouse.io/v1",
  "defaultLbTargetGroupNetworkId": "test",
  "internalNetworkIDs": ["test"],
  "kind": "YandexCloudDiscoveryData",
  "region": "test",
  "routeTableID": "test",
  "shouldAssignPublicIPAddress": false,
  "zoneToSubnetIdMap": {
    "ru-central1-a": "test",
    "ru-central1-b": "test"
  },
  "zones": ["ru-central1-a", "ru-central1-b"]
}`

	validClusterConfigYAML = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.0.0.0/16
sshPublicKey: ssh-rsa AAAAB3...
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd85m9q2qspfnsv055rh
provider:
  cloudID: test-cloud-id
  folderID: test-folder-id
  serviceAccountJSON: '{"id":"test"}'
`
)

var _ = Describe("FilterPCCSecret", func() {
	newPCCSecret := func(name string, data map[string][]byte) *unstructured.Unstructured {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "d8-cloud-provider-yandex",
			},
			Data: data,
		}
		result, err := unstructuredFromObj(secret)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("wrong secret name returns nil", func() {
		result, err := FilterPCCSecret(newPCCSecret("wrong-name", nil))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("invalid cluster-config YAML returns error", func() {
		result, err := FilterPCCSecret(newPCCSecret(PCCSecretName, map[string][]byte{
			PCCClusterConfigFilename: []byte(`}{not valid yaml`),
		}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("validate cloud-provider-cluster-configuration.yaml"))
		Expect(result).To(BeNil())
	})

	It("invalid discovery-data JSON returns error", func() {
		result, err := FilterPCCSecret(newPCCSecret(PCCSecretName, map[string][]byte{
			PCCDiscoveryDataFilename: []byte(`not valid json`),
		}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("validate cloud-provider-discovery-data.json"))
		Expect(result).To(BeNil())
	})

	It("valid PCC secret with both cluster-config and discovery-data", func() {
		result, err := FilterPCCSecret(newPCCSecret(PCCSecretName, map[string][]byte{
			PCCDiscoveryDataFilename: []byte(validDiscoveryData),
			PCCClusterConfigFilename: []byte(validClusterConfigYAML),
		}))
		Expect(err).NotTo(HaveOccurred())
		pccResult, ok := result.(*PCCSecretFilterResult)
		Expect(ok).To(BeTrue())
		Expect(pccResult.ProviderClusterConfig).NotTo(BeNil())
		Expect(pccResult.ProviderDiscoveryData).NotTo(BeNil())
	})

	It("valid PCC secret with only cluster-config", func() {
		result, err := FilterPCCSecret(newPCCSecret(PCCSecretName, map[string][]byte{
			PCCClusterConfigFilename: []byte(validClusterConfigYAML),
		}))
		Expect(err).NotTo(HaveOccurred())
		pccResult, ok := result.(*PCCSecretFilterResult)
		Expect(ok).To(BeTrue())
		Expect(pccResult.ProviderClusterConfig).NotTo(BeNil())
		Expect(pccResult.ProviderDiscoveryData).To(BeNil())
	})

	It("valid PCC secret with only discovery-data", func() {
		result, err := FilterPCCSecret(newPCCSecret(PCCSecretName, map[string][]byte{
			PCCDiscoveryDataFilename: []byte(validDiscoveryData),
		}))
		Expect(err).NotTo(HaveOccurred())
		pccResult, ok := result.(*PCCSecretFilterResult)
		Expect(ok).To(BeTrue())
		Expect(pccResult.ProviderClusterConfig).To(BeNil())
		Expect(pccResult.ProviderDiscoveryData).NotTo(BeNil())
	})
})

var _ = Describe("FilterModuleConfig", func() {
	newMCUnstructured := func(version int, enabled *bool, settings map[string]interface{}) *unstructured.Unstructured {
		mc := &deckhousev1alpha1.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cloud-provider-yandex",
			},
			Spec: deckhousev1alpha1.ModuleConfigSpec{
				Version: version,
			},
		}

		if enabled != nil {
			mc.Spec.Enabled = enabled
		}

		if len(settings) > 0 {
			settingsJSON, err := json.Marshal(settings)
			Expect(err).NotTo(HaveOccurred())
			var mappedFields deckhousev1alpha1.MappedFields
			err = json.Unmarshal(settingsJSON, &mappedFields)
			Expect(err).NotTo(HaveOccurred())
			mc.Spec.Settings = ptr.To(mappedFields)
		}

		result, err := unstructuredFromObj(mc)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("MC v1 with settings returns SettingsV1", func() {
		obj := newMCUnstructured(1, ptr.To(true), map[string]interface{}{
			"additionalExternalNetworkIDs": []interface{}{"net-1"},
			"storageClass": map[string]interface{}{
				"exclude": []interface{}{"network-hdd"},
			},
		})
		result, err := FilterModuleConfig(obj)
		Expect(err).NotTo(HaveOccurred())
		mcResult := result.(ModuleConfigFilterResult)
		Expect(mcResult.Version).To(Equal(int64(1)))
		Expect(mcResult.Enabled).To(BeTrue())
		Expect(mcResult.SettingsV2).To(BeNil())
		Expect(mcResult.SettingsV1).NotTo(BeNil())
		// The payload is decoded into the typed v1 settings, so assert the fields the projection
		// actually reads rather than the raw JSON.
		Expect(mcResult.SettingsV1.AdditionalExternalNetworkIDs).To(Equal([]string{"net-1"}))
		Expect(mcResult.SettingsV1.StorageClass.Exclude).To(Equal([]string{"network-hdd"}))
	})

	It("MC v2 with settings returns SettingsV2", func() {
		obj := newMCUnstructured(2, ptr.To(true), map[string]interface{}{
			"provider": map[string]interface{}{
				"parameters": map[string]interface{}{
					"cloudID":  "cloud-1",
					"folderID": "folder-1",
				},
			},
		})
		result, err := FilterModuleConfig(obj)
		Expect(err).NotTo(HaveOccurred())
		mcResult := result.(ModuleConfigFilterResult)
		Expect(mcResult.Version).To(Equal(int64(2)))
		Expect(mcResult.Enabled).To(BeTrue())
		Expect(mcResult.SettingsV1).To(BeNil())
		Expect(mcResult.SettingsV2).NotTo(BeNil())
		Expect(mcResult.SettingsV2.Provider.Parameters.CloudID).To(Equal("cloud-1"))
		Expect(mcResult.SettingsV2.Provider.Parameters.FolderID).To(Equal("folder-1"))
	})

	It("MC with no settings returns empty settings", func() {
		obj := newMCUnstructured(1, ptr.To(true), nil)
		result, err := FilterModuleConfig(obj)
		Expect(err).NotTo(HaveOccurred())
		mcResult := result.(ModuleConfigFilterResult)
		Expect(mcResult.Version).To(Equal(int64(1)))
		Expect(mcResult.Enabled).To(BeTrue())
		Expect(mcResult.SettingsV1).To(BeNil())
		Expect(mcResult.SettingsV2).To(BeNil())
	})

	It("MC with disabled flag", func() {
		obj := newMCUnstructured(2, ptr.To(false), map[string]interface{}{"x": "y"})
		result, err := FilterModuleConfig(obj)
		Expect(err).NotTo(HaveOccurred())
		mcResult := result.(ModuleConfigFilterResult)
		Expect(mcResult.Version).To(Equal(int64(2)))
		Expect(mcResult.Enabled).To(BeFalse())
		Expect(mcResult.SettingsV2).NotTo(BeNil())
	})
})

var _ = Describe("FilterCredentialSecret", func() {
	newCredentialSecret := func(name string, secretType corev1.SecretType) *unstructured.Unstructured {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "d8-cloud-provider-yandex",
			},
			Type: secretType,
		}
		result, err := unstructuredFromObj(secret)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("secret with correct type returns name", func() {
		result, err := FilterCredentialSecret(
			newCredentialSecret("d8-credentials", cpapi.CredentialsSecretType),
		)
		Expect(err).NotTo(HaveOccurred())
		credResult := result.(NamedResourceFilterResult)
		Expect(credResult.Name).To(Equal("d8-credentials"))
	})

	It("secret with wrong type returns nil", func() {
		result, err := FilterCredentialSecret(
			newCredentialSecret("some-secret", corev1.SecretTypeOpaque),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})
})

var _ = Describe("FilterNamedResource", func() {
	It("returns the object name", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-worker-group",
				},
			},
		}

		result, err := FilterNamedResource(obj)
		Expect(err).NotTo(HaveOccurred())
		namedResult := result.(NamedResourceFilterResult)
		Expect(namedResult.Name).To(Equal("my-worker-group"))
	})
})

var _ = Describe("FilterCandiDiscoverySecret", func() {
	newDiscoverySecret := func(name string, data map[string][]byte) *unstructured.Unstructured {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "d8-cloud-provider-yandex",
			},
			Data: data,
		}
		result, err := unstructuredFromObj(secret)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("wrong name returns nil", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret("wrong-name", nil))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("secret without discovery data key returns empty result", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"other-key": []byte("{}"),
		}))
		Expect(err).NotTo(HaveOccurred())
		discResult := result.(CandiDiscoveryDataFilterResult)
		// nil, not a zero-value struct: the Secret exists long before the infrastructure run
		// fills it in, and ResolveDiscoveryData has to tell "no payload here" from a real
		// payload so that an empty candi Secret does not shadow the legacy PCC discovery data.
		Expect(discResult.ProviderDiscoveryData).To(BeNil())
	})

	It("valid secret with discovery data", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"cloud-provider-discovery-data.json": []byte(validDiscoveryData),
		}))
		Expect(err).NotTo(HaveOccurred())
		discResult := result.(CandiDiscoveryDataFilterResult)
		Expect(discResult.ProviderDiscoveryData).NotTo(Equal(clouddatav1.YandexCloudDiscoveryData{}))
		Expect(discResult.ProviderDiscoveryData.APIVersion).To(Equal("deckhouse.io/v1"))
	})

	// candi/openapi/cloud_discovery_data.yaml is a second schema over the same payload, and
	// it used to require routeTableID, defaultLbTargetGroupNetworkId, internalNetworkIDs,
	// zones and zoneToSubnetIdMap to be non-empty. A cluster whose infrastructure DKP does
	// not create has none of them, and YandexCloudDiscoveryData omits them rather than
	// writing empty values, so both shapes have to get through this filter.
	It("minimal discovery data is accepted", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"cloud-provider-discovery-data.json": []byte(`{
  "apiVersion": "deckhouse.io/v1",
  "kind": "YandexCloudDiscoveryData",
  "region": "ru-central1"
}`),
		}))
		Expect(err).NotTo(HaveOccurred())
		discResult := result.(CandiDiscoveryDataFilterResult)
		Expect(discResult.ProviderDiscoveryData.Region).To(Equal("ru-central1"))
	})

	It("discovery data with empty collections is accepted", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"cloud-provider-discovery-data.json": []byte(`{
  "apiVersion": "deckhouse.io/v1",
  "kind": "YandexCloudDiscoveryData",
  "region": "ru-central1",
  "routeTableID": "",
  "defaultLbTargetGroupNetworkId": "",
  "internalNetworkIDs": [],
  "zones": [],
  "zoneToSubnetIdMap": {}
}`),
		}))
		Expect(err).NotTo(HaveOccurred())
		discResult := result.(CandiDiscoveryDataFilterResult)
		Expect(discResult.ProviderDiscoveryData.Region).To(Equal("ru-central1"))
	})

	It("discovery data without a region is rejected", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"cloud-provider-discovery-data.json": []byte(`{
  "apiVersion": "deckhouse.io/v1",
  "kind": "YandexCloudDiscoveryData"
}`),
		}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("validate candi cloud-provider-discovery-data.json"))
		Expect(result).To(BeNil())
	})

	It("invalid discovery data returns error", func() {
		result, err := FilterCandiDiscoverySecret(newDiscoverySecret(CandiDiscoverySecretName, map[string][]byte{
			"cloud-provider-discovery-data.json": []byte(`{invalid`),
		}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("validate candi cloud-provider-discovery-data.json"))
		Expect(result).To(BeNil())
	})
})

// unstructuredFromObj converts a typed k8s object to *unstructured.Unstructured.
func unstructuredFromObj(obj interface{}) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonBytes, &u.Object)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// findRepoRoot walks up from the current file to locate the deckhouse repository root.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
