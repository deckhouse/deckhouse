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

const (
	initValuesString       = `{"registryPackagesProxy":{"internal":{}}}`
	initConfigValuesString = `{}`
)

func proxyPodManifest(name, podIP string, phase, ready string, terminating bool) string {
	deletionTimestamp := ""
	if terminating {
		deletionTimestamp = "\n  deletionTimestamp: \"2026-08-06T10:00:00Z\""
	}

	return fmt.Sprintf(`
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: d8-cloud-instance-manager
  labels:
    app: registry-packages-proxy%s
status:
  phase: %s
  podIP: %s
  conditions:
  - type: Ready
    status: "%s"
`, name, deletionTimestamp, phase, podIP, ready)
}

var _ = Describe("Module :: registry-packages-proxy :: hooks :: discover proxy addresses ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("publishes an empty address list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("registryPackagesProxy.internal.proxyAddresses").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Two serving pods on different masters", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				proxyPodManifest("registry-packages-proxy-b", "192.168.0.2", "Running", "True", false) +
					proxyPodManifest("registry-packages-proxy-a", "192.168.0.1", "Running", "True", false),
			))
			f.RunHook()
		})

		It("publishes both addresses, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("registryPackagesProxy.internal.proxyAddresses").String()).
				To(MatchJSON(`["192.168.0.1","192.168.0.2"]`))
		})
	})

	Context("Pods that do not serve traffic", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				proxyPodManifest("registry-packages-proxy-serving", "192.168.0.1", "Running", "True", false) +
					proxyPodManifest("registry-packages-proxy-not-ready", "192.168.0.2", "Running", "False", false) +
					proxyPodManifest("registry-packages-proxy-terminating", "192.168.0.3", "Running", "True", true) +
					proxyPodManifest("registry-packages-proxy-completed", "192.168.0.4", "Succeeded", "False", false),
			))
			f.RunHook()
		})

		It("publishes only the address of the serving pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("registryPackagesProxy.internal.proxyAddresses").String()).
				To(MatchJSON(`["192.168.0.1"]`))
		})
	})

	Context("Several pods share one master address", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				proxyPodManifest("registry-packages-proxy-old", "192.168.0.1", "Running", "True", false) +
					proxyPodManifest("registry-packages-proxy-new", "192.168.0.1", "Running", "True", false),
			))
			f.RunHook()
		})

		It("publishes the address once", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("registryPackagesProxy.internal.proxyAddresses").String()).
				To(MatchJSON(`["192.168.0.1"]`))
		})
	})

	Context("On the BeforeHelm event", func() {
		BeforeEach(func() {
			f.KubeStateSet(proxyPodManifest("registry-packages-proxy-a", "192.168.0.1", "Running", "True", false))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("publishes the address discovered from the cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("registryPackagesProxy.internal.proxyAddresses").String()).
				To(MatchJSON(`["192.168.0.1"]`))
		})
	})
})
