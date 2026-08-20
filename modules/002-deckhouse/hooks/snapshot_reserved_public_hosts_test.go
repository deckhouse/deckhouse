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
	"os"
	"regexp"
	"sort"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// policyResources pulls the group and the resource out of every call the template makes to its
// policy definition, which is the list of what the admission policies deny.
var policyResources = regexp.MustCompile(
	`reserved_public_hosts_policy" \(list \. \$paramsName "[a-z]+" "([a-z0-9.]+)" "([a-z]+)"`)

// TestTheSnapshotReadsEveryResourceThePoliciesDeny keeps the two sides of the grandfathering in
// step. Whatever the policies deny, the snapshot has to be able to record: a resource on one side
// and not the other means a tenant is denied on its next write with nothing in grandfatheredHosts to
// let it back out.
//
// Compared against the template rather than against a literal, so that adding a policy without
// adding the resource here fails, which is the drift worth catching.
func TestTheSnapshotReadsEveryResourceThePoliciesDeny(t *testing.T) {
	template, err := os.ReadFile("../templates/reserved-public-hosts.yaml")
	if err != nil {
		t.Fatalf("read the policy template: %v", err)
	}

	denied := []string{}
	for _, match := range policyResources.FindAllStringSubmatch(string(template), -1) {
		denied = append(denied, match[1]+"/"+match[2])
	}
	if len(denied) == 0 {
		t.Fatal("no policy call sites matched, the pattern no longer fits the template")
	}

	read := []string{}
	for _, resource := range hostBearingResources {
		read = append(read, resource.gr.Group+"/"+resource.gr.Resource)
	}

	sort.Strings(denied)
	sort.Strings(read)
	if len(denied) != len(read) {
		t.Fatalf("the policies deny %v, the snapshot reads %v", denied, read)
	}
	for i := range denied {
		if denied[i] != read[i] {
			t.Errorf("the policies deny %v, the snapshot reads %v", denied, read)
			break
		}
	}
}

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
kind: GRPCRoute
metadata:
  name: rpc
  namespace: tenant
spec:
  hostnames:
  - rpc.example.com
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TLSRoute
metadata:
  name: sni
  namespace: tenant
spec:
  hostnames:
  - sni.example.com
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
	f.RegisterCRD("gateway.networking.k8s.io", "v1", "GRPCRoute", true)
	// The only version upstream serves TLSRoute under, which is also why the hook resolves the
	// version from discovery instead of pinning one.
	f.RegisterCRD("gateway.networking.k8s.io", "v1alpha2", "TLSRoute", true)
	f.RegisterCRD("gateway.networking.k8s.io", "v1", "ListenerSet", true)
	// Gateway is missing on purpose: the fake cluster derives the resource name from the kind with
	// meta.UnsafeGuessKindToResource, which turns Gateway into "gatewaies" and not "gateways", so a
	// Gateway registered here would be served under a name no cluster uses. It reads its hostnames
	// through the same function as ListenerSet, and that the hook asks for the resource the policies
	// target is checked by TestTheSnapshotReadsEveryResourceThePoliciesDeny below.

	// An empty template is passed by leaving the value out: the global schema requires a %s, so an
	// empty string never reaches a cluster, and setting one here would fail values validation.
	run := func(domainTemplate, kubeState string) {
		if domainTemplate != "" {
			f.ValuesSet("global.modules.publicDomainTemplate", domainTemplate)
		}
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
				"hosts": [
					"docs.example.com",
					"portal.example.com",
					"rpc.example.com",
					"shop.example.com",
					"sni.example.com"
				]
			}`))
		})

		It("reads every kind that can carry a hostname", func() {
			// One hostname per kind. A kind the policies deny but this hook does not read leaves the
			// tenant serving it denied on its next write with nothing in the record to let it back
			// out, which is the promise the grandfathering makes.
			hosts := f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()
			for kind, host := range map[string]string{
				"Ingress":     "shop.example.com",
				"HTTPRoute":   "docs.example.com",
				"GRPCRoute":   "rpc.example.com",
				"TLSRoute":    "sni.example.com",
				"ListenerSet": "portal.example.com",
			} {
				Expect(hosts).To(ContainSubstring(host), kind)
			}
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

		// The wildcard is reserved by exact match, which the allowlist this record feeds cannot lift,
		// so recording it would only put an entry in the record that the policies ignore -- and an
		// operator reading the record would take it for a hostname that still works. Holding the
		// wildcard shadows every hostname the template renders, including the ones the platform
		// learns to publish later, so it is the one claim the upgrade does not carry over.
		It("leaves it out, because the record cannot give the wildcard back", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": []
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

	Context("An operator hand-edited the record into something that is not a hostname", func() {
		BeforeEach(func() {
			run("%s.example.com", tenantObjects+paramsConfigMap("true",
				"    Shop.example.com\n"+
					"    store.example.com.\n"+
					"    https://admin.example.com\n"+
					"    -not-a-host-.example.com\n"))
		})

		// The header of the hook points an operator at this key, and its value goes straight into
		// deckhouse.internal.reservedPublicHosts.hosts, which openapi/values.yaml validates. A typo
		// there must not stop the module that renders Deckhouse from converging, so what is a
		// hostname is kept, spelled the way the policies compare it, and the rest is dropped.
		It("keeps what is a hostname and drops the rest rather than failing values validation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["shop.example.com", "store.example.com"]
			}`))
		})
	})

	// The record is applied only under Template mode, so recording under List would snapshot a moment
	// the reservation it feeds was not in force at: a cluster installed on List and switched to
	// Template a year later would then find a record from its List days and grandfather nothing that
	// appeared in between.
	Context("A cluster asks for the reservation the module shipped before", func() {
		BeforeEach(func() {
			f.ValuesSet("deckhouse.reservedPublicHosts.mode", "List")
			run("%s.example.com", tenantObjects)
		})

		It("records nothing while the reservation the record feeds is not in force", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": false,
				"hosts": []
			}`))
		})
	})

	Context("A cluster that asked for Template mode explicitly", func() {
		BeforeEach(func() {
			f.ValuesSet("deckhouse.reservedPublicHosts.mode", "Template")
			run("%s.example.com", tenantObjects)
		})

		It("records, which is what makes the switch from List take a snapshot of the day it happens", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath + ".recorded").Bool()).To(BeTrue())
			Expect(f.ValuesGet(reservedPublicHostsValuePath + ".hosts").String()).
				To(ContainSubstring("shop.example.com"))
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

	// A template the global schema rejects, such as one with two %s, cannot be exercised here:
	// values validation refuses it before the hook runs. The hooks/lib/publicdomain package covers that
	// ParseNamespace yields nothing for such a value, which is what makes the hook record nothing
	// instead of guessing at a namespace.
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

	// The policies match apiVersions ["*"], so the snapshot has to read whichever version the cluster
	// serves. A version pinned here would leave a cluster whose Gateway API came from elsewhere with
	// nothing recorded while every write to those objects is still denied.
	Context("The Gateway API is served under an older version", func() {
		f := HookExecutionConfigInit(`{"global": {"modules": {}}, "deckhouse": {"internal": {}}}`, `{}`)
		f.RegisterCRD("gateway.networking.k8s.io", "v1beta1", "HTTPRoute", true)

		BeforeEach(func() {
			f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: tenant
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: docs
  namespace: tenant
spec:
  hostnames:
  - docs.example.com
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("reads the version the cluster serves rather than one pinned in the hook", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(reservedPublicHostsValuePath).String()).To(MatchJSON(`{
				"recorded": true,
				"hosts": ["docs.example.com"]
			}`))
		})
	})
})
