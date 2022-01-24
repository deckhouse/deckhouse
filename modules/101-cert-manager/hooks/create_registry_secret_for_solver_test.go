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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	testDockerCfgEncoded = `YQo=`
)

func genTestChallengeManifest(name, ns string) string {
	return fmt.Sprintf(`
apiVersion: acme.cert-manager.io/v1
kind: Challenge
metadata:
  labels:
    acme.cert-manager.io/order-name: candi-dashboard-308008487
  name: "%s"
  namespace: "%s"
  ownerReferences:
    - apiVersion: acme.cert-manager.io/v1
      blockOwnerDeletion: true
      controller: true
      kind: Order
      name: some_name
      uid: 87233806-25b3-41b4-8c15-46b7212326b4
spec:
  authzURL: https://acme-v02.api.letsencrypt.org/acme/authz-v3/000000000
  config:
    http01:
      ingressClass: nginx
  dnsName: some.domain
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  key: some_key
  token: some_token
  type: http-01
  url: https://acme-v02.api.letsencrypt.org/acme/chall-v3/000000000/aaaaaa
  wildcard: false
`, name, ns)
}

// todo remove with legacy cert-manager
func genTestLegacyChallengeManifest(name, ns string) string {
	return fmt.Sprintf(`
apiVersion: certmanager.k8s.io/v1alpha1
kind: Challenge
metadata:
  labels:
    acme.cert-manager.io/order-name: candi-dashboard-308008487
  name: "%s"
  namespace: "%s"
  ownerReferences:
    - apiVersion: certmanager.k8s.io/v1alpha1
      blockOwnerDeletion: true
      controller: true
      kind: Order
      name: some_name
      uid: 87233806-25b3-41b4-8c15-46b7212326b4
spec:
  authzURL: https://acme-v02.api.letsencrypt.org/acme/authz-v3/000000000
  config:
    http01:
      ingressClass: nginx
  dnsName: some.domain
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  key: some_key
  token: some_token
  type: http-01
  url: https://acme-v02.api.letsencrypt.org/acme/chall-v3/000000000/aaaaaa
  wildcard: false
`, name, ns)
}

func genD8RegistrySecret(secretContent string) string {
	return genRegistrySecret(secretContent, "d8-system", "deckhouse-registry")
}

func genRegistrySecret(secretContent, ns, name string) string {
	return fmt.Sprintf(`
apiVersion: v1
data:
  .dockerconfigjson: %s
kind: Secret
metadata:
  name: %s
  namespace: %s
type: kubernetes.io/dockerconfigjson`, secretContent, name, ns)
}

func genServiceAccount(ns string) string {
	return fmt.Sprintf(`
apiVersion: v1
imagePullSecrets:
- name: %s
kind: ServiceAccount
metadata:
  labels:
    cert-manager.deckhouse.io/solver-sa: "true"
    heritage: deckhouse
  name: %s
  namespace: %s`, solverSecretName, solverServiceAccountName, ns)
}

func setState(f *HookExecutionConfig, waitForQuantity int, ch ...string) {
	rs := strings.Join(ch, "\n---")
	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(rs, waitForQuantity))
}

func assertRegistrySecretAndSAExists(f *HookExecutionConfig, dockerCfgContent string, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, solverSecretName)
		Expect(secret).To(Not(BeEmpty()))
		config := secret.Field(`data`).Get("\\.dockerconfigjson").String()
		// yes decoded, because we use SecretTypeDockerConfigJson
		Expect(config).To(BeEquivalentTo(dockerCfgContent))

		sa := f.KubernetesResource("ServiceAccount", ns, solverServiceAccountName)
		Expect(sa).ToNot(BeEmpty())
	}
}

func assertRegistrySecretAndSANotExists(f *HookExecutionConfig, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, solverSecretName)
		Expect(secret).To(BeEmpty())

		sa := f.KubernetesResource("ServiceAccount", ns, solverServiceAccountName)
		Expect(sa).To(BeEmpty())
	}
}

var _ = Describe("Cert Manager hooks :: generate registry secret for http challenge solver ::", func() {
	f := HookExecutionConfigInit(`{"global":{}}`, "")
	f.RegisterCRD("acme.cert-manager.io", "v1", "Challenge", true)
	// todo remove with legacy cert-manager
	f.RegisterCRD("certmanager.k8s.io", "v1alpha1", "Challenge", true)

	const ns1 = "ns1"
	const ns2 = "ns2"
	const ns3 = "ns3"
	const chName = "chName"
	const chNameAnother = "chName2"

	Context("Empty cluster", func() {
		BeforeEach(func() {
			setState(f, 0, ``)

			f.RunHook()
		})

		It("runs successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	genChallengeFunc := []struct {
		fun   func(name, ns string) string
		title string
	}{
		{genTestChallengeManifest, "New cert-manager manifests"},
		{genTestLegacyChallengeManifest, "Old cert-manager manifests"},
	}

	for _, tst := range genChallengeFunc {
		genChallengeManifest := tst.fun
		Context(tst.title, func() {
			Context("Deckhouse registry secret", func() {
				BeforeEach(func() {
					setState(f, 1, genD8RegistrySecret(testDockerCfgEncoded))

					f.RunHook()
				})

				Context("only created", func() {
					It("runs successfully", func() {
						Expect(f).To(ExecuteSuccessfully())
					})
				})

				Context("add challenges", func() {
					BeforeEach(func() {
						setState(f, 4,
							genD8RegistrySecret(testDockerCfgEncoded),

							genChallengeManifest(chName, ns1),
							genChallengeManifest(chName, ns2),
							genChallengeManifest(chName, ns3),
							genChallengeManifest(chNameAnother, ns3),
						)

						f.RunHook()
					})

					Context("d8 registry secret content changing", func() {
						const newContent = "enhjCg=="
						BeforeEach(func() {
							Expect(newContent).ToNot(Equal(testDockerCfgEncoded))

							setState(f, 5,
								genD8RegistrySecret(newContent),

								genChallengeManifest(chName, ns1),
								genChallengeManifest(chName, ns2),
								genChallengeManifest(chName, ns3),
								genChallengeManifest(chNameAnother, ns3),
							)

							f.RunHook()
						})

						It("changes secret content for all solvers secrets", func() {
							assertRegistrySecretAndSAExists(f, newContent, ns1, ns2, ns3)
						})
					})
				})
			})

			Context("Creating", func() {
				BeforeEach(func() {
					setState(f, 2,
						genD8RegistrySecret(testDockerCfgEncoded),
						genChallengeManifest(chName, ns1),
					)

					f.RunHook()
					Expect(f).To(ExecuteSuccessfully())
				})

				Context("one challenge in one namespace", func() {
					It("creates registry secret", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1)
						assertRegistrySecretAndSANotExists(f, ns2, ns3)
					})
				})

				Context("multiple challenges in same namespace", func() {
					BeforeEach(func() {
						setState(f, 1,
							genD8RegistrySecret(testDockerCfgEncoded),
							genChallengeManifest(chName, ns1),
							genChallengeManifest(chNameAnother, ns1),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("contains one registry secret", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1)
						assertRegistrySecretAndSANotExists(f, ns2, ns3)
					})
				})

				Context("one challenge per namespace", func() {
					BeforeEach(func() {
						setState(f, 4,
							genD8RegistrySecret(testDockerCfgEncoded),
							genChallengeManifest(chName, ns1),
							genChallengeManifest(chName, ns2),
							genChallengeManifest(chName, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("creates one secret in each namespace", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
					})
				})
			})

			Context("Deleting", func() {
				BeforeEach(func() {
					setState(f, 1,
						genD8RegistrySecret(testDockerCfgEncoded),
						genChallengeManifest(chName, ns1),
						genChallengeManifest(chName, ns2),
						genChallengeManifest(chName, ns3),
						genChallengeManifest(chNameAnother, ns3),
					)

					f.RunHook()
					Expect(f).To(ExecuteSuccessfully())
				})

				Context("last challenge in one namespace", func() {
					BeforeEach(func() {
						setState(f, 7,
							genD8RegistrySecret(testDockerCfgEncoded),
							genChallengeManifest(chName, ns2),
							genChallengeManifest(chName, ns3),
							genChallengeManifest(chNameAnother, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("deletes registry secret in challenge namespace", func() {
						assertRegistrySecretAndSANotExists(f, ns1)
					})

					It("keeps registry secret in another namespaces", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns2, ns3)
					})
				})

				Context("not last challenge in one namespace", func() {
					BeforeEach(func() {
						setState(f, 1,
							genD8RegistrySecret(testDockerCfgEncoded),
							genChallengeManifest(chName, ns1),
							genChallengeManifest(chName, ns2),
							genChallengeManifest(chName, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("keeps registry secret", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns3)
					})

					It("keeps registry secret in another namespaces", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2)
					})
				})
			})
		})
	}

	Context("Registry secret created without service account", func() {
		BeforeEach(func() {
			setState(f, 1,
				genD8RegistrySecret(testDockerCfgEncoded),

				genTestChallengeManifest(chName, ns1),
				genRegistrySecret(testDockerCfgEncoded, ns1, solverSecretName),
			)

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should create service account", func() {
			assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1)
		})
	})

	Context("Service account created without registry secret", func() {
		BeforeEach(func() {
			setState(f, 1,
				genD8RegistrySecret(testDockerCfgEncoded),

				genTestChallengeManifest(chName, ns1),
				genServiceAccount(ns1),
			)

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should create registry secret", func() {
			assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1)
		})
	})

	Context("Challenge deleted", func() {
		Context("Service account was deleted but secret was not", func() {
			BeforeEach(func() {
				setState(f, 1,
					genD8RegistrySecret(testDockerCfgEncoded),

					genRegistrySecret(testDockerCfgEncoded, ns1, solverSecretName),
				)

				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())
			})

			It("Should delete secret", func() {
				assertRegistrySecretAndSANotExists(f, ns1)
			})
		})

		Context("Secret was deleted but service account was not", func() {
			BeforeEach(func() {
				setState(f, 1,
					genD8RegistrySecret(testDockerCfgEncoded),

					genServiceAccount(ns1),
				)

				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())
			})

			It("Should delete service account", func() {
				assertRegistrySecretAndSANotExists(f, ns1)
			})
		})
	})

	// todo remove with legacy cert-manager
	Context("Legacy cert-manager manifests and new cert manager manifests both", func() {
		Context("Removing", func() {
			BeforeEach(func() {
				setState(f, 7,
					genD8RegistrySecret(testDockerCfgEncoded),

					genTestChallengeManifest(chName, ns1),
					genTestChallengeManifest(chName, ns2),
					genTestChallengeManifest(chName, ns3),

					genTestLegacyChallengeManifest(chNameAnother, ns1),
					genTestLegacyChallengeManifest(chNameAnother, ns2),
					genTestLegacyChallengeManifest(chNameAnother, ns3),
				)
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())
			})

			It("create registry secret in all namespaces", func() {
				assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
			})

			Context("remove legacy challenge in one namespace", func() {
				BeforeEach(func() {
					setState(f, 1,
						genD8RegistrySecret(testDockerCfgEncoded),

						genTestChallengeManifest(chName, ns1),
						genTestChallengeManifest(chName, ns2),
						genTestChallengeManifest(chName, ns3),

						genTestLegacyChallengeManifest(chNameAnother, ns1),
						genTestLegacyChallengeManifest(chNameAnother, ns2),
					)

					f.RunHook()

					Expect(f).To(ExecuteSuccessfully())
				})

				It("keeps registry secret in all namespaces", func() {
					assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
				})

				Context("remove new challenge in one namespace", func() {
					BeforeEach(func() {
						setState(f, 7,
							genD8RegistrySecret(testDockerCfgEncoded),

							genTestChallengeManifest(chName, ns1),
							genTestChallengeManifest(chName, ns2),

							genTestLegacyChallengeManifest(chNameAnother, ns1),
							genTestLegacyChallengeManifest(chNameAnother, ns2),
						)

						f.RunHook()

						Expect(f).To(ExecuteSuccessfully())
					})

					It("removes registry secret from namespace", func() {
						assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2)
						assertRegistrySecretAndSANotExists(f, ns3)
					})
				})
			})

			Context("remove legacy challenges in all namespace", func() {
				BeforeEach(func() {
					setState(f, 3,
						genD8RegistrySecret(testDockerCfgEncoded),

						genTestChallengeManifest(chName, ns1),
						genTestChallengeManifest(chName, ns2),
						genTestChallengeManifest(chName, ns3),
					)

					f.RunHook()

					Expect(f).To(ExecuteSuccessfully())
				})

				It("keeps registry secret in all namespaces", func() {
					assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
				})
			})

			Context("remove new challenges in all namespaces", func() {
				BeforeEach(func() {
					setState(f, 3,
						genD8RegistrySecret(testDockerCfgEncoded),

						genTestLegacyChallengeManifest(chNameAnother, ns1),
						genTestLegacyChallengeManifest(chNameAnother, ns2),
						genTestLegacyChallengeManifest(chNameAnother, ns3),
					)

					f.RunHook()

					Expect(f).To(ExecuteSuccessfully())
				})

				It("keeps registry secret in all namespaces", func() {
					assertRegistrySecretAndSAExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
				})
			})

			Context("remove all challenges in all namespaces", func() {
				BeforeEach(func() {
					setState(f, 12,
						genD8RegistrySecret(testDockerCfgEncoded),
					)

					f.RunHook()

					Expect(f).To(ExecuteSuccessfully())
				})

				It("removes secrets in all namespaces", func() {
					assertRegistrySecretAndSANotExists(f, ns1, ns2, ns3)
				})
			})
		})
	})
})
