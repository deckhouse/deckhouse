// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var kubernetesCA = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIIJGfeJReTrQYwDQYJKoZIhvcNAQELBQAwFTETMBEGA1UE
AxMKa3ViZXJuZXRlczAeFw0yMzA4MjcwODMyMDFaFw0zMzA4MjQwODMyMDFaMBUx
EzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQC550LJfqE+5zvO4o/maCtEQW+6H13r95pjaEv/RSb+iBFIqyZ9SBLBVIdO
6WUo8y4gRqfY5OeMwwIM8Hy+8TGr6hWAtE7pOfvLFc2nwU0mnzqyOlry8FhWM6o8
UR3ysfMk1HBaI4zbD4xBXdS1KoFgGo1uvj+GTWZ8cRyiBv88S6gK7WqAJ41jg/J5
3z73FJvgLPPzS8MDtTE4ORWpYGRQqjd0kLpQcpbeiih2FyNNoqV1bhmOTrDmdm8r
NtY8wKPoLIeGCV7dXnfEtA6QoQjQLIhu2cMZjH6sqQ3PQkaUuRvtOGQecFPuPkHD
OroXYggpLOsmF6rS2gcRBy87h9jXAgMBAAGjWTBXMA4GA1UdDwEB/wQEAwICpDAP
BgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRbCvugzxrznaRyCGWs7bPub953QDAV
BgNVHREEDjAMggprdWJlcm5ldGVzMA0GCSqGSIb3DQEBCwUAA4IBAQA4WsM8PHcc
FkzfFvuYgDkMDTaA0V421TPrPZ/lq/8shoNDnpUJPBpQALrB89LhUr0ER2kXlRwv
SwVaDQBxgfL47gax9FAWx/VRvMX/t5Yjob8R/76RbejhDSQJgav9XZ/uH5X15u7M
Gw3wuDjfk+Axmua+hnunN22PQ2iCnvLeBjUw4j0GfCs9mT3+mycEyDV5KoTL7sNf
Hu83kkG0TzimIAMYi2XbQ8Yhbv6uEYhTRhvf3Os9MO8iwQk6NIWj6i5EnI/uDD6g
1vVRE+aaM9+HwnpQz9Kqm0JpE8PawaPg+wApKFOS/e9cRsSrZylS6kKG5m41j+LS
RDeV3R5SRnfD
-----END CERTIFICATE-----`

var _ = Describe("Control-plane-manager :: kube-scheduler-extenders-test", func() {
	kubernetesCABase64 := base64.StdEncoding.EncodeToString([]byte(kubernetesCA))
	f := HookExecutionConfigInit(`{"global": {"discovery": {"clusterDomain": "cluster.local"}}, "controlPlaneManager": {"internal": {"kubeSchedulerExtenders": [{}]}}}`, `{}`)

	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "kubeschedulerwebhookconfigurations"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "KubeSchedulerWebhookConfiguration", false)

	var (
		extender1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: test1
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: test1
      port: 8080
      path: /scheduler
    caBundle: ` + kubernetesCABase64 + `
  timeoutSeconds: 5
`
		extender2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: test2
webhooks:
- weight: 10
  failurePolicy: Fail
  clientConfig:
    service:
      name: scheduler
      namespace: test2
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
- weight: 20
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler2
      namespace: test2
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 10
`
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must be empty", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(BeZero())
		})

	})

	Context("Add one extender, CA is valid", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesCA", kubernetesCA)
			f.BindingContexts.Set(f.KubeStateSet(extender1))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must contain one element", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(Equal(1))
			Expect(f.ValuesGet(configPath).Array()[0].String()).To(MatchJSON(`
          {
            "urlPrefix": "https://scheduler.test1.svc.cluster.local:8080/scheduler",
            "weight": 5,
            "timeout": 5,
            "ignorable": true,
            "caData": "` + kubernetesCABase64 + `"
          }
`))
		})

	})

	Context("Add three extenders, CA is invalid", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesCA", kubernetesCA)
			f.BindingContexts.Set(f.KubeStateSet(extender1 + extender2))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must contain three elements", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(Equal(3))
			Expect(f.ValuesGet(configPath).Array()[0].String()).To(MatchJSON(`
          {
            "urlPrefix": "https://scheduler.test1.svc.cluster.local:8080/scheduler",
            "weight": 5,
            "timeout": 5,
            "ignorable": true,
            "caData": "` + kubernetesCABase64 + `"
}`))

			Expect(f.ValuesGet(configPath).Array()[1].String()).To(MatchJSON(`
          {
            "urlPrefix": "https://scheduler.test2.svc.cluster.local:8080/scheduler",
            "weight": 10,
            "timeout": 5,
            "ignorable": false,
            "caData": "` + kubernetesCABase64 + `"
          }`))

			Expect(f.ValuesGet(configPath).Array()[2].String()).To(MatchJSON(`
          {
            "urlPrefix": "https://scheduler2.test2.svc.cluster.local:8080/scheduler",
            "weight": 20,
            "timeout": 10,
            "ignorable": true,
            "caData": "` + kubernetesCABase64 + `"
          }`))

		})

	})

})
