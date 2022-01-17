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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func assertCleanMetricsOnly(f *HookExecutionConfig) {
	ops := f.MetricsCollector.CollectedMetrics()
	Expect(len(ops)).To(BeEquivalentTo(1))

	// first is expiration
	Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
		Group:  metricsGroup,
		Action: "expire",
	}))
}

var _ = Describe("Modules :: cert-manager :: hooks :: orphan_secrets_metrics ::", func() {
	const (
		stateCertificates = `
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  annotations:
    meta.helm.sh/release-name: dashboard
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: dashboard
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: dashboard
  name: dashboard
  namespace: non-d8-dashboard
spec:
  acme:
    config:
    - domains:
      - dashboard.test
      http01:
        ingressClass: nginx
  dnsNames:
  - dashboard.test
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: ingress-tls
`
		stateSecrets = `
---
apiVersion: v1
data:
  ca.crt: ""
  tls.crt: LS0tLS1C
  tls.key: LS0tLS1C
kind: Secret
metadata:
  annotations:
    cert-manager.io/alt-names: dashboard.test
    cert-manager.io/certificate-name: dashboard
    cert-manager.io/common-name: dashboard.test
    cert-manager.io/ip-sans: ""
    cert-manager.io/issuer-kind: ClusterIssuer
    cert-manager.io/issuer-name: letsencrypt
  name: ingress-tls
  namespace: non-d8-dashboard
type: kubernetes.io/tls
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("cert-manager.io", "v1", "Certificate", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("Secret in cluster, Certificate not in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecrets))
			f.RunHook()
		})

		It("adds orphan metrics for group", func() {
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(2))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  metricsGroup,
				Action: "expire",
			}))

			// second is metrics
			expectedMetric := operation.MetricOperation{
				Name:   "d8_orphan_secrets_without_corresponding_certificate_resources",
				Group:  metricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"namespace":   "non-d8-dashboard",
					"secret_name": "ingress-tls",
					"annotation":  "cert-manager.io/certificate-name",
				},
			}
			Expect(ops[1]).To(BeEquivalentTo(expectedMetric))
		})
	})

	Context("Secret in cluster, Certificate in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCertificates + stateSecrets))
			f.RunHook()
		})

		It("expire orphan metrics for group only", func() {
			assertCleanMetricsOnly(f)
		})
	})

	Context("Certificate in cluster, secret is not in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCertificates))
			f.RunHook()
		})

		It("expire orphan metrics for group only", func() {
			assertCleanMetricsOnly(f)
		})
	})
})
