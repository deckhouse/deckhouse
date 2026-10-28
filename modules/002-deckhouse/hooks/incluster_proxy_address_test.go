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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// inclusterProxyPodState is an in-cluster proxy Pod as the hook sees one: the labels it selects
// on, the address of the node it runs on, and whether it is serving.
func inclusterProxyPodState(name, hostIP string, ready bool) string {
	condition := "False"
	if ready {
		condition = "True"
	}

	return fmt.Sprintf(`
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: d8-system
  labels:
    app: registry-incluster-proxy
    component: registry-service
    type: incluster-proxy
spec:
  hostNetwork: true
status:
  hostIP: %s
  conditions:
  - type: Ready
    status: "%s"
`, name, hostIP, condition)
}

// deckhouseDeployment is the Pod spec this hook reads back: whatever the alias currently resolves
// `registry.d8-system.svc` to, which is the answer a restart has to be able to recover without
// the values it lost.
func deckhouseDeployment(aliasIP string) string {
	return fmt.Sprintf(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      hostAliases:
      - ip: %q
        hostnames:
        - registry.d8-system.svc
`, aliasIP)
}

const inclusterProxyAddressValue = "deckhouse.internal.inclusterProxyAddress"

var _ = Describe("deckhouse :: hooks :: incluster_proxy_address ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse":{"internal":{}}}`, ``)

	Context("A single proxy Pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true),
			))
			f.RunHook()
		})

		It("points the alias at its node, not at the loopback address", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).String()).To(Equal("192.168.199.233"))
		})
	})

	Context("Several proxy Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true) +
					inclusterProxyPodState("registry-incluster-proxy-1", "192.168.199.100", true) +
					inclusterProxyPodState("registry-incluster-proxy-2", "192.168.199.50", true),
			))
			f.RunHook()
		})

		// Writing this value restarts the Deckhouse Pod, so the same set of Pods has to produce
		// the same answer however the snapshot happens to be ordered. Alphabetically, which puts
		// ".100" ahead of ".50" -- not an order anyone would call natural, only a fixed one.
		It("takes the first address, and always the same one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).String()).To(Equal("192.168.199.100"))
		})
	})

	Context("An alias that still reaches a proxy", func() {
		BeforeEach(func() {
			// ".233" is deliberately NOT the one a fresh choice would make: ".100" sorts ahead
			// of it.
			f.BindingContexts.Set(f.KubeStateSet(
				deckhouseDeployment("192.168.199.233") +
					inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true) +
					inclusterProxyPodState("registry-incluster-proxy-1", "192.168.199.100", true),
			))
			f.RunHook()
		})

		// Republishing rewrites this Pod's spec and restarts it for an address that answers no
		// better. This is the recovery case too: the values are empty, as they are after any
		// restart, and the answer still has to come out as what is already rendered.
		It("is kept, even though it is not the one a fresh choice would pick", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).String()).To(Equal("192.168.199.233"))
		})
	})

	Context("An alias whose proxy has gone", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				deckhouseDeployment("192.168.199.7") +
					inclusterProxyPodState("registry-incluster-proxy-1", "192.168.199.100", true) +
					inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true),
			))
			f.RunHook()
		})

		It("moves to the first address", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).String()).To(Equal("192.168.199.100"))
		})
	})

	Context("An alias still on the loopback fallback", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				deckhouseDeployment("127.0.0.1") +
					inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true),
			))
			f.RunHook()
		})

		// The loopback address answers only on the node the proxy happens to be on, which is the
		// failure this hook exists for -- so it is not an address worth keeping.
		It("moves off it to the node running the proxy", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).String()).To(Equal("192.168.199.233"))
		})
	})

	Context("The only proxy Pod is not Ready", func() {
		BeforeEach(func() {
			f.ValuesSet(inclusterProxyAddressValue, "192.168.199.233")
			f.BindingContexts.Set(f.KubeStateSet(
				inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", false),
			))
			f.RunHook()
		})

		It("publishes nothing, leaving the template on its loopback fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).Exists()).To(BeFalse())
		})
	})

	Context("A bootstrapped cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet(inclusterProxyAddressValue, "192.168.199.233")
			f.BindingContexts.Set(f.KubeStateSet(
				inclusterProxyPodState("registry-incluster-proxy-0", "192.168.199.233", true),
			))
			f.RunHook()
		})

		// The alias is not rendered at all past bootstrap, so there is no address to keep.
		It("drops the address it published earlier", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(inclusterProxyAddressValue).Exists()).To(BeFalse())
		})
	})
})
