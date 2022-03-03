/*
Copyright 2022 Flant JSC

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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/v1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: generate exporter api key ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  cloudMetricsExporterEnabled: true
  internal:
    exporter: {}
`
	)

	providerConfiguration := func(sa string) string {
		return fmt.Sprintf(`
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    imageID: test
    memory: 4096
  replicas: 1
nodeNetworkCIDR: 10.231.0.0/22
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    %s
sshPublicKey: ssh-rsa test
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`, sa)
	}

	providerSecret := func(conf string) string {
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(conf)))
	}

	providerSecretForSa := func(sa string) string {
		return providerSecret(providerConfiguration(sa))
	}

	exporterSecret := func(folderId, apiKey, checksum, keyId string) string {
		apiKeySecret := ""
		if apiKey != "" {
			apiKeySecret = fmt.Sprintf(`"api-key": %s`, base64.StdEncoding.EncodeToString([]byte(apiKey)))
		}
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-yandex-metrics-exporter-app-creds
  namespace: d8-monitoring
  annotations:
    checksum/service-account: "%s"
    service-account-api-key/id: "%s"
data:
  "folder-id": "%s"
  %s
`, checksum, keyId, base64.StdEncoding.EncodeToString([]byte(folderId)), apiKeySecret)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	marshaledKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	privateKeyBuf := new(bytes.Buffer)

	err = pem.Encode(privateKeyBuf, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: marshaledKey,
	})

	if err != nil {
		panic(err)
	}

	privateKeyBytes, err := io.ReadAll(privateKeyBuf)
	if err != nil {
		panic(err)
	}

	serviceAccount := func(id, saID string) (string, string) {
		sa := map[string]interface{}{
			"id":                 id,
			"service_account_id": saID,
			"created_at":         "2020-08-17T08:56:17Z",
			"key_algorithm":      "RSA_2048",
			"public_key":         "public key",
			"private_key":        string(privateKeyBytes),
		}

		res, _ := json.Marshal(sa)

		return string(res), fmt.Sprintf("%x", sha256.Sum256(res))
	}

	assertCheckValues := func(f *HookExecutionConfig, apiKey apiKeySecret) {
		k := f.ValuesGet("cloudProviderYandex.internal.exporter.apiKey").String()
		Expect(k).To(Equal(apiKey.Key))

		s := f.ValuesGet("cloudProviderYandex.internal.exporter.serviceAccountChecksum").String()
		Expect(s).To(Equal(apiKey.ServiceAccountChecksum))

		i := f.ValuesGet("cloudProviderYandex.internal.exporter.apiKeyID").String()
		Expect(i).To(Equal(apiKey.KeyID))
	}

	requestedCreateAPIKeysForSA := make(map[string]struct{})
	requestedDeleteAPIKeysForSA := make(map[string]struct{})

	AfterEach(func() {
		requestedCreateAPIKeysForSA = make(map[string]struct{})
	})

	mockRequests := func(apiKey, keyID string) {
		dependency.TestDC.HTTPClient.DoMock.
			Set(func(req *http.Request) (rp1 *http.Response, err error) {
				if req.Method == "DELETE" {
					path := strings.Split(req.URL.Path, "/")
					if len(path) != 5 {
						return nil, fmt.Errorf("incerrect DELETE path")
					}
					requestedDeleteAPIKeysForSA[path[4]] = struct{}{}
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(nil)),
					}, nil
				}

				switch req.URL.String() {
				case "https://iam.api.cloud.yandex.net/iam/v1/tokens":
					resp := v1.IAMTokenCreationResponse{IAMToken: "iam token"}
					respBytes, _ := json.Marshal(resp)
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
					}, nil
				case "https://iam.api.cloud.yandex.net/iam/v1/apiKeys":
					var reqBody v1.APIKeyCreationRequest
					err := json.NewDecoder(req.Body).Decode(&reqBody)
					if err != nil {
						return nil, err
					}
					requestedCreateAPIKeysForSA[reqBody.ServiceAccountID] = struct{}{}
					resp := v1.APIKeyCreationResponse{
						APIKey: v1.APIKeyResponse{
							ID:               keyID,
							ServiceAccountID: reqBody.ServiceAccountID,
							CreatedAt:        "2020-08-17T08:56:17Z",
							Description:      reqBody.Description,
						},
						Secret: apiKey,
					}
					respBytes, _ := json.Marshal(resp)
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
					}, nil
				}

				return nil, fmt.Errorf("incorrect url")
			})
	}

	f := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("hook should fail with errors", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Provider cluster configuration is successfully discovered", func() {
		Context("Service account is empty", func() {
			BeforeEach(func() {
				JoinKubeResourcesAndSet(f, providerSecretForSa(""))

				f.RunHook()
			})

			It("hook should fail with errors", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
			})

			It("does not request new api-key", func() {
				Expect(f).ToNot(ExecuteSuccessfully())

				Expect(requestedCreateAPIKeysForSA).To(HaveLen(0))
			})
		})

		Context("Secret with exporter credentials not present", func() {
			id := "id1"
			saID := "saID1"
			apiKey := "apikey" + saID
			sa, checksum := serviceAccount(id, saID)
			keyID := "apiKeyId1"
			BeforeEach(func() {
				mockRequests(apiKey, keyID)

				JoinKubeResourcesAndSet(f, providerSecretForSa(sa))

				f.RunHook()
			})

			It("requests new api-key", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(requestedCreateAPIKeysForSA).To(HaveKey(saID))
			})

			It("set api-key and service-account checksum into values", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertCheckValues(f, apiKeySecret{
					Key:                    apiKey,
					KeyID:                  keyID,
					ServiceAccountChecksum: checksum,
				})
			})
		})

		Context("Secret with exporter credentials present", func() {
			id := "id2"
			saID := "sa2"
			apiKey := "apikey" + saID
			sa, checksum := serviceAccount(id, saID)
			keyID := "apiKeyId2"

			Context("api key not present in secret", func() {
				BeforeEach(func() {
					mockRequests(apiKey, keyID)

					JoinKubeResourcesAndSet(f, providerSecretForSa(sa), exporterSecret("folder", "", checksum, keyID))

					f.RunHook()
				})

				It("requests new api-key", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(requestedCreateAPIKeysForSA).To(HaveKey(saID))
				})

				It("set api-key and service-account checksum into values", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertCheckValues(f, apiKeySecret{
						Key:                    apiKey,
						KeyID:                  keyID,
						ServiceAccountChecksum: checksum,
					})
				})
			})

			Context("service account in provider cluster configuration not changed", func() {
				id := "id3"
				saID := "sa3"
				apiKey := "apikey" + saID
				sa, checksum := serviceAccount(id, saID)
				keyID := "apiKeyId3"

				BeforeEach(func() {
					mockRequests(apiKey, keyID)

					JoinKubeResourcesAndSet(f, providerSecretForSa(sa), exporterSecret("folder", apiKey, checksum, keyID))

					f.RunHook()
				})

				It("does not request new api-key", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(requestedCreateAPIKeysForSA).To(HaveLen(0))
				})

				It("set api-key and service-account checksum into values", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertCheckValues(f, apiKeySecret{
						Key:                    apiKey,
						KeyID:                  keyID,
						ServiceAccountChecksum: checksum,
					})
				})
			})

			Context("service account in provider cluster configuration was changed", func() {
				id := "id4"
				saID := "sa4"
				apiKey := "apikey" + saID
				sa, checksum := serviceAccount(id, saID)
				keyID := "apiKeyId4"

				BeforeEach(func() {
					mockRequests(apiKey, keyID)

					JoinKubeResourcesAndSet(f, providerSecretForSa(sa), exporterSecret("folder", apiKey, "oldchecksum", keyID))

					f.RunHook()
				})

				It("request new api-key", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(requestedCreateAPIKeysForSA).To(HaveKey(saID))
				})

				It("set api-key and service-account checksum into values", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertCheckValues(f, apiKeySecret{
						Key:                    apiKey,
						KeyID:                  keyID,
						ServiceAccountChecksum: checksum,
					})
				})
			})
		})
	})

	Context("Exporter deploying disabled from config", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderYandex.cloudMetricsExporterEnabled", false)
		})

		Context("secret with api-key does not exists", func() {
			id := "id3"
			saID := "sa3"
			apiKey := "apikey" + saID
			sa, _ := serviceAccount(id, saID)
			keyID := "apiKeyId3"

			BeforeEach(func() {
				mockRequests(apiKey, keyID)

				JoinKubeResourcesAndSet(f, providerSecretForSa(sa))

				f.RunHook()
			})

			It("does not request request creating api-key", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(requestedCreateAPIKeysForSA).To(HaveLen(0))
			})

			It("does not request request deleting api-key", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(requestedDeleteAPIKeysForSA).To(HaveLen(0))
			})

			It("clean values", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertCheckValues(f, apiKeySecret{})
			})
		})

		Context("secret with api-key exists", func() {
			id := "id7"
			saID := "sa7"
			apiKey := "apikey7" + saID
			sa, checksum := serviceAccount(id, saID)
			keyID := "apiKeyId7"

			BeforeEach(func() {
				mockRequests(apiKey, keyID)

				JoinKubeResourcesAndSet(f, providerSecretForSa(sa), exporterSecret("folder", apiKey, checksum, keyID))

				f.RunHook()
			})

			It("does not request request creating api-key", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(requestedCreateAPIKeysForSA).To(HaveLen(0))
			})

			It("requests deleting api-key", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(requestedDeleteAPIKeysForSA).To(HaveKey(keyID))
			})

			It("clean values", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertCheckValues(f, apiKeySecret{})
			})
		})
	})
})
