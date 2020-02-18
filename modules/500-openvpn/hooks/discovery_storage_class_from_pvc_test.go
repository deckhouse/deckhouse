package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: openvpn :: hooks :: discovery_storage_class_from_pvc ::", func() {
	const (
		initValuesString       = `{"openvpn":{"internal": {}}}`
		initConfigValuesString = `{"openvpn":{}}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: openvpn
  labels:
    app: openvpn
spec:
  storageClassName: gp2
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("cluster with pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc))
			f.RunHook()
		})

		It("currentStorageClass must be gp2", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.internal.currentStorageClass").String()).To(Equal("gp2"))
		})
	})

})
