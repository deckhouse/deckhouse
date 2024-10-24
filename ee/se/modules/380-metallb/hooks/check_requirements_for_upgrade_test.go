/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	l2Advertisement = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-1
  - pool-2
  nodeSelectors:
  - matchLabels:
      zone: a
`
	ipAddressPolls = `
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
spec:
  addresses:
  - 11.11.11.11/32
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-2
  namespace: d8-metallb
spec:
  addresses:
  - 22.22.22.22/32
`
	ipAddressPolls2 = `
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
spec:
  addresses:
  - 11.11.11.11/32
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-3
  namespace: d8-metallb
spec:
  addresses:
  - 33.33.33.33/32
`
	l2Advertisement2 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-b
  namespace: metallb
spec:
  ipAddressPools:
  - pool-1
  nodeSelectors:
  - matchLabels:
      zone: b
`
	l2Advertisement3 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-2
  nodeSelectors:
  - matchLabels:
      zone: a
`
	l2Advertisement4 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-b
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-1
  nodeSelectors:
  - matchExpressions: []
  - matchLabels:
      zone: b
`
)

var _ = Describe("Metallb hooks :: check requirements for upgrade ::", func() {
	f := HookExecutionConfigInit(`{}`, `{"global":{"discovery":{}}}`)
	f.RegisterCRD("metallb.io", "v1beta1", "L2Advertisement", true)
	f.RegisterCRD("metallb.io", "v1beta1", "IPAddressPool", true)

	Context("Check correct configurations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(l2Advertisement + ipAddressPolls))
			f.RunHook()
		})
		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("OK"))
		})
	})

	Context("Check addressPollsMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(l2Advertisement + ipAddressPolls2))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("addressPollsMismatch"))
		})
	})

	Context("Check nsMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(l2Advertisement2 + l2Advertisement3))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("nsMismatch"))
		})
	})

	Context("Check nodeSelectorsMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(l2Advertisement3 + l2Advertisement4))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("nodeSelectorsMismatch"))
		})
	})
})
