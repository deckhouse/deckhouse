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
	testDockerCfg        = `YQ==`
	testDockerCfgDecoded = `a`
)

var testInitClusterConfigForSolverSecret = fmt.Sprintf(`
{
	"global":{
		"modulesImages":{
			"registryDockercfg":"%s"
		}
	}
}`, testDockerCfg)

func genTestChallengeManifest(name, ns string) string {
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

func setTestChallenges(f *HookExecutionConfig, ch ...string) {
	rs := strings.Join(ch, "\n---")
	f.BindingContexts.Set(f.KubeStateSet(rs))
}

func assertRegistrySecretExists(f *HookExecutionConfig, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, solverSecretName)
		Expect(secret).To(Not(BeEmpty()))
		config := secret.Field(`stringData`).Get("\\.dockerconfigjson").String()
		// yes decoded, because we use SecretTypeDockerConfigJson
		Expect(config).To(BeEquivalentTo(testDockerCfgDecoded))
	}
}

func assertRegistrySecretNotExists(f *HookExecutionConfig, nss ...string) {
	for _, ns := range nss {
		secret := f.KubernetesResource("Secret", ns, solverSecretName)
		Expect(secret).To(BeEmpty())
	}
}

var _ = Describe("Cert Manager hooks :: generate registry secret for http challenge solver ::", func() {
	f := HookExecutionConfigInit(testInitClusterConfigForSolverSecret, "")
	f.RegisterCRD("certmanager.k8s.io", "v1alpha1", "Challenge", true)

	const ns1 = "ns1"
	const ns2 = "ns2"
	const ns3 = "ns3"
	const chName = "chName"
	const chNameAnother = "chName2"

	Context("Creating", func() {
		BeforeEach(func() {
			setTestChallenges(f,
				genTestChallengeManifest(chName, ns1),
			)

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("one challenge in one namespace", func() {
			It("creates registry secret", func() {
				assertRegistrySecretExists(f, ns1)
				assertRegistrySecretNotExists(f, ns2, ns3)
			})
		})

		Context("multiple challenges in same namespace", func() {
			BeforeEach(func() {
				setTestChallenges(f,
					genTestChallengeManifest(chName, ns1),
					genTestChallengeManifest(chNameAnother, ns1),
				)

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("contains one registry secret", func() {
				assertRegistrySecretExists(f, ns1)
				assertRegistrySecretNotExists(f, ns2, ns3)
			})
		})

		Context("one challenge per namespace", func() {
			BeforeEach(func() {
				setTestChallenges(f,
					genTestChallengeManifest(chName, ns1),
					genTestChallengeManifest(chName, ns2),
					genTestChallengeManifest(chName, ns3),
				)

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("creates one secret in each namespace", func() {
				assertRegistrySecretExists(f, ns1, ns2, ns3)
			})
		})
	})

	Context("Deleting", func() {
		BeforeEach(func() {
			setTestChallenges(f,
				genTestChallengeManifest(chName, ns1),
				genTestChallengeManifest(chName, ns2),
				genTestChallengeManifest(chName, ns3),
				genTestChallengeManifest(chNameAnother, ns3),
			)

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("last challenge in one namespace", func() {
			BeforeEach(func() {
				setTestChallenges(f,
					genTestChallengeManifest(chName, ns2),
					genTestChallengeManifest(chName, ns3),
					genTestChallengeManifest(chNameAnother, ns3),
				)

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("deletes registry secret in challenge namespace", func() {
				assertRegistrySecretNotExists(f, ns1)
			})

			It("keeps registry secret in another namespaces", func() {
				assertRegistrySecretExists(f, ns2, ns3)
			})
		})

		Context("not last challenge in one namespace", func() {
			BeforeEach(func() {
				setTestChallenges(f,
					genTestChallengeManifest(chName, ns1),
					genTestChallengeManifest(chName, ns2),
					genTestChallengeManifest(chName, ns3),
				)

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("keeps registry secret", func() {
				assertRegistrySecretExists(f, ns3)
			})

			It("keeps registry secret in another namespaces", func() {
				assertRegistrySecretExists(f, ns1, ns2)
			})
		})
	})
})
