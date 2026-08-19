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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// tenantObjects is a cluster where a tenant already serves hostnames the reservation is about to
// claim, next to hostnames it must leave alone and a platform namespace it must never read.
const tenantObjects = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: tenant
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-user-authn
  labels:
    heritage: deckhouse
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shop
  namespace: tenant
spec:
  rules:
  - host: shop.example.com
  - host: shop.example.org
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ecosystem
  namespace: tenant
spec:
  rules:
  - host: app.ns.example.com
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: no-host
  namespace: tenant
spec:
  rules:
  - http:
      paths: []
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dex
  namespace: d8-user-authn
spec:
  rules:
  - host: dex.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: docs
  namespace: tenant
spec:
  hostnames:
  - Docs.Example.COM.
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: listeners
  namespace: tenant
spec:
  listeners:
  - name: portal
    hostname: portal.example.com
  - name: any
    port: 80
`

// paramsConfigMap is the object Helm renders and the hook reads its own record back out of.
func paramsConfigMap(recorded, hosts string) string {
	return `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
  labels:
    heritage: deckhouse
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-reserved-public-hosts
  namespace: d8-system
  labels:
    heritage: deckhouse
data:
  grandfatherRecorded: "` + recorded + `"
  grandfatheredHosts: |
` + hosts + `
`
}

var _ = Describe("Modules :: deckhouse :: hooks :: snapshot reserved public hosts ::", func() {
	f := HookExecutionConfigInit(`{"global": {"modules": {}}, "deckhouse": {"internal": {}}}`, `{}`)
	f.RegisterCRD("gateway.networking.k8s.io", "v1", "HTTPRoute", true)
	f.RegisterCRD("gateway.networking.k8s.io", "v1", "ListenerSet", true)

	run := func(domainTemplate, kubeState string) {
		f.ValuesSet("global.modules.publicDomainTemplate", domainTemplate)
		f.KubeStateSet(kubeState)
		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()
	}

	Context("A cluster where the reservation has never been recorded", func() {
		BeforeEach(func() {
			run("%s.example.com", tenantObjects)
		})

		It("records the hostnames the reservation is about to claim, and only those", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["docs.example.com", "portal.example.com", "shop.example.com"]
			}`))
		})

		It("reads every kind that can carry a hostname", func() {
			// One from each: an Ingress rule, an HTTPRoute hostname and a ListenerSet listener. A
			// kind left out would leave the tenant serving it denied on its next write.
			hosts := f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()
			Expect(hosts).To(ContainSubstring("shop.example.com"))
			Expect(hosts).To(ContainSubstring("docs.example.com"))
			Expect(hosts).To(ContainSubstring("portal.example.com"))
		})

		It("spells a hostname the way the policies compare it", func() {
			// Docs.Example.COM. in the cluster: the policies lowercase and drop the root dot before
			// comparing, so a record that kept either would never be found.
			Expect(f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()).
				To(ContainSubstring(`"docs.example.com"`))
		})

		It("leaves out what the reservation was never going to claim", func() {
			hosts := f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()
			Expect(hosts).NotTo(ContainSubstring("shop.example.org"), "another domain")
			Expect(hosts).NotTo(ContainSubstring("app.ns.example.com"), "two labels, an application hostname")
			Expect(hosts).NotTo(ContainSubstring("dex.example.com"),
				"served from a heritage: deckhouse namespace, which the policies never match either")
		})
	})

	Context("A tenant already holds the wildcard of the platform's domain", func() {
		BeforeEach(func() {
			run("%s.example.com", `
---
apiVersion: v1
kind: Namespace
metadata:
  name: tenant
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: catch-all
  namespace: tenant
spec:
  rules:
  - host: "*.example.com"
  - host: "*.other.example.com"
`)
		})

		It("records it, since the reservation is about to claim that too", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["*.example.com"]
			}`))
		})
	})

	Context("The reservation has already been recorded", func() {
		BeforeEach(func() {
			run("%s.example.com", tenantObjects+paramsConfigMap("true", "    shop.example.com\n"))
		})

		// The property the whole hook exists for. A second snapshot would turn the allowlist into one
		// that widens itself: claim a hostname, wait for the next converge, keep it.
		It("keeps the record it made and does not take in what appeared since", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["shop.example.com"]
			}`))
		})
	})

	Context("An operator pruned the record down to nothing", func() {
		BeforeEach(func() {
			run("%s.example.com", tenantObjects+paramsConfigMap("true", ""))
		})

		// An empty record is a legitimate state and must not read as "not recorded yet", which is
		// why the ConfigMap carries the flag as a key of its own.
		It("leaves it empty rather than filling it in again", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": []
			}`))
		})
	})

	Context("The parameters exist but say the record has not been made", func() {
		BeforeEach(func() {
			run("%s.example.com", tenantObjects+paramsConfigMap("false", ""))
		})

		It("makes it, which is what an upgrade into Template mode needs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath + ".recorded").Bool()).To(BeTrue())
			Expect(f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()).
				To(ContainSubstring("shop.example.com"))
		})
	})

	Context("The platform publishes nothing yet", func() {
		BeforeEach(func() {
			run("", tenantObjects)
		})

		It("records nothing and does not claim to have recorded", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Nothing is reserved without a template, so there is nothing to grandfather -- and the
			// flag has to stay down, or a template set later would find a record that covers none of
			// what tenants were serving when it started.
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": false,
				"hosts": []
			}`))
		})
	})

	Context("A publicDomainTemplate the global schema would have rejected", func() {
		BeforeEach(func() {
			run("%s-%s.example.com", tenantObjects)
		})

		It("records nothing rather than guessing at a namespace", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": false,
				"hosts": []
			}`))
		})
	})

	Context("The template puts the service name inside the first label", func() {
		BeforeEach(func() {
			run("kube-%s.company.my", `
---
apiVersion: v1
kind: Namespace
metadata:
  name: tenant
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shop
  namespace: tenant
spec:
  rules:
  - host: kube-shop.company.my
  - host: shop.company.my
  - host: "*.company.my"
`)
		})

		It("records what that template covers and nothing beside it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["kube-shop.company.my"]
			}`))
		})
	})

	Context("The cluster has no Gateway API", func() {
		f := HookExecutionConfigInit(`{"global": {"modules": {}}, "deckhouse": {"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: tenant
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shop
  namespace: tenant
spec:
  rules:
  - host: shop.example.com
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("records the Ingress hostnames instead of failing on the kinds that are missing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["shop.example.com"]
			}`))
		})
	})
})
