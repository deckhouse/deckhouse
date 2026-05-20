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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: alert_on_deprecated_dex_provider_tls ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "DexProvider", false)

	expireOp := operation.MetricOperation{
		Group:  "d8_dex_provider_ldap_tls_conflict",
		Action: operation.ActionExpireMetrics,
	}
	setOp := func(name, conflict string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   "d8_dex_provider_ldap_tls_conflict",
			Value:  ptr.To(1.0),
			Labels: map[string]string{"name": name, "conflict": conflict},
			Action: operation.ActionGaugeSet,
			Group:  "d8_dex_provider_ldap_tls_conflict",
		}
	}

	Context("No DexProvider objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Expires metric, sets nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(ConsistOf(expireOp))
		})
	})

	Context("DexProvider with type OIDC", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: OIDC
  displayName: GitLab
  oidc:
    clientID: abc
    clientSecret: xyz
    issuer: https://gitlab.example.com
`))
			f.RunHook()
		})
		It("Expires metric, sets nothing for non-LDAP providers", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})

	Context("LDAP provider with valid configuration (no conflict)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: ldap-ok
spec:
  type: LDAP
  displayName: ok
  ldap:
    host: ldap.example.org:389
    startTLS: true
    insecureSkipVerify: true
    userSearch:
      baseDN: ou=People,dc=x
      username: uid
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
`))
			f.RunHook()
		})
		It("Expires metric, sets nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})

	Context("LDAP provider with insecureNoSSL + startTLS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: ldap-bad-starttls
spec:
  type: LDAP
  displayName: bad
  ldap:
    host: ldap.example.org:389
    insecureNoSSL: true
    startTLS: true
    userSearch:
      baseDN: ou=People,dc=x
      username: uid
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
`))
			f.RunHook()
		})
		It("Emits conflict metric with label insecureNoSSL+startTLS", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOp))
			Expect(m[1]).To(BeEquivalentTo(setOp("ldap-bad-starttls", "insecureNoSSL+startTLS")))
		})
	})

	Context("LDAP provider with insecureNoSSL + insecureSkipVerify + rootCAData", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: ldap-bad-multi
spec:
  type: LDAP
  displayName: bad
  ldap:
    host: ldap.example.org:389
    insecureNoSSL: true
    insecureSkipVerify: true
    rootCAData: "-----BEGIN CERTIFICATE-----\nMIIFaDC...\n-----END CERTIFICATE-----\n"
    userSearch:
      baseDN: ou=People,dc=x
      username: uid
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
`))
			f.RunHook()
		})
		It("Emits two conflict metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[0]).To(BeEquivalentTo(expireOp))
			Expect(m[1:]).To(ConsistOf(
				setOp("ldap-bad-multi", "insecureNoSSL+insecureSkipVerify"),
				setOp("ldap-bad-multi", "insecureNoSSL+rootCAData"),
			))
		})
	})

	Context("Conflict is resolved on the next reconcile", func() {
		const bad = `
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: ldap-flip
spec:
  type: LDAP
  displayName: flip
  ldap:
    host: ldap.example.org:389
    insecureNoSSL: true
    startTLS: true
    userSearch:
      baseDN: ou=People,dc=x
      username: uid
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
`
		const fixed = `
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: ldap-flip
spec:
  type: LDAP
  displayName: flip
  ldap:
    host: ldap.example.org:389
    insecureNoSSL: true
    userSearch:
      baseDN: ou=People,dc=x
      username: uid
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(bad))
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(fixed))
			f.RunHook()
		})
		It("Last run expires the conflict metric and emits nothing else", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m[len(m)-1]).To(BeEquivalentTo(expireOp))
		})
	})
})
