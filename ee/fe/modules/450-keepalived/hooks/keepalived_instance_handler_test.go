/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"keepalived":{"instances": {}}}`
	initConfigValuesString = `{}`
)

const (
	keepalivedInstance = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front1
spec:
  nodeSelector:
    node-role/frontend: ""
  vrrpInstances:
  - id: 11
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 1.1.1.1/32
  - id: 12
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 1.2.2.2/32
`
	keepalivedInstanceWithNotUniqueID = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: not-unique
spec:
  nodeSelector:
    node-role/not-unique: ""
  vrrpInstances:
  - id: 12
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 4.3.2.1/32
`

	keepalivedInstanceWithSomeSelectors = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front-1
spec:
  nodeSelector:
    node-role/frontend: ""
    node-role/test: "test"
  vrrpInstances:
  - id: 11
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 1.1.1.1/32
  - id: 12
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 1.2.2.2/32
`
	nodeOne = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-1
  labels:
    node-role/frontend: ""
`
	nodeTwo = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-2
  labels:
    node-role/frontend: ""
    node-role/loadbalancer: "1"
    node-role/test: "test"
`
	nodeThree = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-3
  labels:
    node-role/frontend: ""
`
	nodeFour = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-4
  labels:
    node-role/frontend: ""
    node-role/loadbalancer: "2"
    node-role/asxsa: "kjsds"
`
	secret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: keepalived-instance-secret-front1
  namespace: d8-keepalived
  labels:
    app: keepalived
    keepalived-instance: front1
type: Opaque
data:
  authPass: MTIz
`
)

var _ = Describe("Keepalived hooks :: keepalived instance handler ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "KeepalivedInstance", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Single keepalived instance in empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(keepalivedInstance))
			f.RunHook()
		})

		It("replicas for instance front1 must be zero; authPass for front1 must be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("keepalived.instances.front1.replicas").String()).To(Equal("0"))
			Expect(len(f.ValuesGet("keepalived.instances.front1.authPass").String())).To(Equal(8))
		})
	})

	Context("Two keepalived instances with non-unique vrrpInstances[].id", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(keepalivedInstance + keepalivedInstanceWithNotUniqueID))
			f.RunHook()
		})

		It("hook must fail with error message", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: All vrrpInstances\[\]\.id in all KeepalivedInstances must be unique cluster-wide\.`))
		})
	})

	Context("Keepalived instance and one node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeOne + keepalivedInstance))
			f.RunHook()
		})

		It("replicas for instance front1 must be one; authPass for front1 must be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("keepalived.instances.front1.replicas").String()).To(Equal("1"))
			Expect(len(f.ValuesGet("keepalived.instances.front1.authPass").String())).To(Equal(8))
		})
	})

	Context("Two nodes + secret + keepalived instance", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeOne + nodeTwo + secret + keepalivedInstance))
			f.RunHook()
		})

		It("replicas for instance front1 must be two; authPass for front1 must be 123", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("keepalived.instances.front1.replicas").String()).To(Equal("2"))
			Expect(f.ValuesGet("keepalived.instances.front1.authPass").String()).To(Equal("123"))
		})
	})

	Context("Four nodes with multiple labels and keepalived instance with multiple node selectors", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeOne + nodeTwo + nodeThree + nodeFour + keepalivedInstanceWithSomeSelectors))
			f.RunHook()
		})

		It("replicas for instance front1 must be one; authPass for front1 must be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("keepalived.instances.front-1.replicas").String()).To(Equal("1"))
			Expect(len(f.ValuesGet("keepalived.instances.front-1.authPass").String())).To(Equal(8))
		})
	})

})
