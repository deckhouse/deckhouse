/*
Copyright 2023 Flant JSC

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

func genTestVMIManifest(name, ns string) string {
	return fmt.Sprintf(`
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: "%s"
  namespace: "%s"
spec:
  running: true
  template: {}
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

func setState(f *HookExecutionConfig, ch ...string) {
	rs := strings.Join(ch, "\n---")
	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(rs, 0))
}

func assertRegistrySecretExists(f *HookExecutionConfig, dockerCfgContent string, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, virtRegistrySecretName)
		Expect(secret).To(Not(BeEmpty()))
		config := secret.Field(`data`).Get("\\.dockerconfigjson").String()
		// yes decoded, because we use SecretTypeDockerConfigJson
		Expect(config).To(BeEquivalentTo(dockerCfgContent))
	}
}

func assertRegistrySecretNotExists(f *HookExecutionConfig, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, virtRegistrySecretName)
		Expect(secret).To(BeEmpty())
	}
}

var _ = Describe("Kubevirt hooks :: generate registry secret for virtual machine instance ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("kubevirt.io", "v1", "VirtualMachineInstance", true)

	// Set Kind for binding.
	createRegistrySecretForVMIHookConfig.Kubernetes[0].Kind = "VirtualMachineInstance"
	createRegistrySecretForVMIHookConfig.Kubernetes[0].ApiVersion = "kubevirt.io/v1"

	const ns1 = "ns1"
	const ns2 = "ns2"
	const ns3 = "ns3"
	const vmiName = "vmiName"
	const vmiNameAnother = "vmiName2"

	Context("Empty cluster", func() {
		BeforeEach(func() {
			setState(f, ``)

			f.RunHook()
		})

		It("runs successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	genVMIFunc := []struct {
		fun   func(name, ns string) string
		title string
	}{
		{genTestVMIManifest, "New kubevirt manifests"},
	}

	for _, tst := range genVMIFunc {
		genVMIManifest := tst.fun
		Context(tst.title, func() {
			Context("Deckhouse registry secret", func() {
				BeforeEach(func() {
					setState(f, genD8RegistrySecret(testDockerCfgEncoded))

					f.RunHook()
				})

				Context("only created", func() {
					It("runs successfully", func() {
						Expect(f).To(ExecuteSuccessfully())
					})
				})

				Context("add challenges", func() {
					BeforeEach(func() {
						setState(f,
							genD8RegistrySecret(testDockerCfgEncoded),

							genVMIManifest(vmiName, ns1),
							genVMIManifest(vmiName, ns2),
							genVMIManifest(vmiName, ns3),
							genVMIManifest(vmiNameAnother, ns3),
						)

						f.RunHook()
					})

					Context("d8 registry secret content changing", func() {
						const newContent = "enhjCg=="
						BeforeEach(func() {
							Expect(newContent).ToNot(Equal(testDockerCfgEncoded))

							setState(f,
								genD8RegistrySecret(newContent),

								genVMIManifest(vmiName, ns1),
								genVMIManifest(vmiName, ns2),
								genVMIManifest(vmiName, ns3),
								genVMIManifest(vmiNameAnother, ns3),
							)

							f.RunHook()
						})

						It("changes secret content for all solvers secrets", func() {
							assertRegistrySecretExists(f, newContent, ns1, ns2, ns3)
						})
					})
				})
			})

			Context("Creating", func() {
				BeforeEach(func() {
					setState(f,
						genD8RegistrySecret(testDockerCfgEncoded),
						genVMIManifest(vmiName, ns1),
					)

					f.RunHook()
					Expect(f).To(ExecuteSuccessfully())
				})

				Context("one challenge in one namespace", func() {
					It("creates registry secret", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns1)
						assertRegistrySecretNotExists(f, ns2, ns3)
					})
				})

				Context("multiple challenges in same namespace", func() {
					BeforeEach(func() {
						setState(f,
							genD8RegistrySecret(testDockerCfgEncoded),
							genVMIManifest(vmiName, ns1),
							genVMIManifest(vmiNameAnother, ns1),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("contains one registry secret", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns1)
						assertRegistrySecretNotExists(f, ns2, ns3)
					})
				})

				Context("one challenge per namespace", func() {
					BeforeEach(func() {
						setState(f,
							genD8RegistrySecret(testDockerCfgEncoded),
							genVMIManifest(vmiName, ns1),
							genVMIManifest(vmiName, ns2),
							genVMIManifest(vmiName, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("creates one secret in each namespace", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns1, ns2, ns3)
					})
				})
			})

			Context("Deleting", func() {
				BeforeEach(func() {
					setState(f,
						genD8RegistrySecret(testDockerCfgEncoded),
						genVMIManifest(vmiName, ns1),
						genVMIManifest(vmiName, ns2),
						genVMIManifest(vmiName, ns3),
						genVMIManifest(vmiNameAnother, ns3),
					)

					f.RunHook()
					Expect(f).To(ExecuteSuccessfully())
				})

				Context("last challenge in one namespace", func() {
					BeforeEach(func() {
						setState(f,
							genD8RegistrySecret(testDockerCfgEncoded),
							genVMIManifest(vmiName, ns2),
							genVMIManifest(vmiName, ns3),
							genVMIManifest(vmiNameAnother, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("deletes registry secret in challenge namespace", func() {
						assertRegistrySecretNotExists(f, ns1)
					})

					It("keeps registry secret in another namespaces", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns2, ns3)
					})
				})

				Context("not last challenge in one namespace", func() {
					BeforeEach(func() {
						setState(f,
							genD8RegistrySecret(testDockerCfgEncoded),
							genVMIManifest(vmiName, ns1),
							genVMIManifest(vmiName, ns2),
							genVMIManifest(vmiName, ns3),
						)

						f.RunHook()
						Expect(f).To(ExecuteSuccessfully())
					})

					It("keeps registry secret", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns3)
					})

					It("keeps registry secret in another namespaces", func() {
						assertRegistrySecretExists(f, testDockerCfgEncoded, ns1, ns2)
					})
				})
			})
		})
	}

})
