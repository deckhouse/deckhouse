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

/*

User-stories: Hook must discover the Gateway API CRDs installed in the cluster together with the versions they define, and save them to .Values.global.discovery.gatewayAPICRDs

*/

package hooks

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// publishedCRDs is the list the hook is expected to publish, in order. It is spelled out here rather
// than taken from the hook so that adding or dropping a CRD has to be a deliberate change in both.
var publishedCRDs = []string{
	"backendtlspolicies.gateway.networking.k8s.io",
	"gatewayclasses.gateway.networking.k8s.io",
	"gateways.gateway.networking.k8s.io",
	"grpcroutes.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",
	"inferencepools.inference.networking.k8s.io",
	"listenersets.gateway.networking.k8s.io",
	"referencegrants.gateway.networking.k8s.io",
	"tcproutes.gateway.networking.k8s.io",
	"tlsroutes.gateway.networking.k8s.io",
	"udproutes.gateway.networking.k8s.io",
}

// crdVersion is one entry of a CRD's spec.versions. Deprecation is a fixture-only field: the hook
// drops deprecated versions, so one never appears in an expectation.
type crdVersion struct {
	name       string
	deprecated bool
	served     bool
	storage    bool
}

// crdManifest builds a CustomResourceDefinition from its full name, splitting off the group so a CRD
// outside gateway.networking.k8s.io can be declared the same way.
func crdManifest(name, kind string, versions ...crdVersion) string {
	plural, group, _ := strings.Cut(name, ".")

	manifest := fmt.Sprintf(`
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: %s
spec:
  group: %s
  names:
    kind: %s
    plural: %s
  scope: Namespaced
  versions:
`, name, group, kind, plural)

	for _, version := range versions {
		manifest += fmt.Sprintf(`  - name: %s
    deprecated: %t
    served: %t
    storage: %t
`, version.name, version.deprecated, version.served, version.storage)
	}

	return manifest
}

// expectedValue renders the whole discovery value: every published CRD in order, carrying the
// versions given for it and none for the rest.
func expectedValue(versionsByCRD map[string][]crdVersion) string {
	entries := make([]string, 0, len(publishedCRDs))
	for _, name := range publishedCRDs {
		versions := make([]string, 0, len(versionsByCRD[name]))
		for _, version := range versionsByCRD[name] {
			versions = append(versions, fmt.Sprintf(`{"name": %q, "served": %t, "storage": %t}`,
				version.name, version.served, version.storage))
		}
		entries = append(entries, fmt.Sprintf(`{"name": %q, "versions": [%s]}`, name, strings.Join(versions, ",")))
	}

	return "[" + strings.Join(entries, ",") + "]"
}

var _ = Describe("Global hooks :: discovery :: gateway_api_crd_available ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	v1 := crdVersion{name: "v1", served: true, storage: true}
	v1beta1 := crdVersion{name: "v1beta1", served: true}
	deprecatedV1alpha2 := crdVersion{name: "v1alpha2", served: true, deprecated: true}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		// Every CRD is still listed, so a consumer never has to tell "not installed" apart from
		// "this hook does not know about it".
		It("lists every CRD with no versions", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.gatewayAPICRDs")).To(MatchJSON(
				expectedValue(nil)))
		})
	})

	Context("Gateway API CRDs are installed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				crdManifest("gateways.gateway.networking.k8s.io", "Gateway", v1, v1beta1) +
					crdManifest("httproutes.gateway.networking.k8s.io", "HTTPRoute", v1) +
					crdManifest("referencegrants.gateway.networking.k8s.io", "ReferenceGrant", v1beta1) +
					crdManifest("backendtlspolicies.gateway.networking.k8s.io", "BackendTLSPolicy", v1) +
					crdManifest("inferencepools.inference.networking.k8s.io", "InferencePool", v1)))
			f.RunHook()
		})

		It("reports each installed CRD's versions", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.gatewayAPICRDs")).To(MatchJSON(
				expectedValue(map[string][]crdVersion{
					"gateways.gateway.networking.k8s.io":           {v1, v1beta1},
					"httproutes.gateway.networking.k8s.io":         {v1},
					"referencegrants.gateway.networking.k8s.io":    {v1beta1},
					"backendtlspolicies.gateway.networking.k8s.io": {v1},
					"inferencepools.inference.networking.k8s.io":   {v1},
				})))
		})
	})

	// The deprecation cases are siblings of the one above rather than nested in it: shrinking the
	// fake cluster from its CRDs to one relies on Deleted events that race with the end of the state
	// change, so a nested case could still see some of them installed.

	// A deprecated version is dropped rather than reported, so nothing downstream has to know
	// what deprecation means.
	Context("A CRD also declares a deprecated version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				crdManifest("gateways.gateway.networking.k8s.io", "Gateway", v1, deprecatedV1alpha2)))
			f.RunHook()
		})

		It("leaves the deprecated version out", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.gatewayAPICRDs")).To(MatchJSON(
				expectedValue(map[string][]crdVersion{
					"gateways.gateway.networking.k8s.io": {v1},
				})))
		})
	})

	// Dropping every version it has leaves the CRD indistinguishable from one that is absent,
	// which is what a consumer should conclude: there is nothing here it may use.
	Context("A CRD declares nothing but deprecated versions", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				crdManifest("gateways.gateway.networking.k8s.io", "Gateway", deprecatedV1alpha2)))
			f.RunHook()
		})

		It("reports it with no versions at all", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.gatewayAPICRDs")).To(MatchJSON(expectedValue(nil)))
		})
	})

	// The hook reports what the CRD declares instead of deciding what counts as usable, so a version
	// that is present but not served is published as such and the consumer applies its own rule.
	Context("A CRD version is not served", func() {
		unservedV1 := crdVersion{name: "v1", served: false, storage: true}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				crdManifest("gateways.gateway.networking.k8s.io", "Gateway", unservedV1)))
			f.RunHook()
		})

		It("reports the version with served false rather than omitting it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.gatewayAPICRDs")).To(MatchJSON(
				expectedValue(map[string][]crdVersion{
					"gateways.gateway.networking.k8s.io": {unservedV1},
				})))
		})
	})
})
