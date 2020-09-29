/*

User-stories:
1. If ConfigMap kubeadm-config exists in kube-system namespace, hook will delete them.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: kubeadm_config_cleanup ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = `{"controlPlaneManager":{"apiserver": {"auditPolicyEnabled": "false"}}}`
		stateA                 = `
apiVersion: v1 
kind: ConfigMap 
metadata:
  name: kubeadm-config
  namespace: kube-system
data:
  foo: bar
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

	})

	Context("Cluster started with stateA ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("ConfigMap kubeadm-config from namespace kube-system must be removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "kubeadm-config").Exists()).To(BeFalse())
		})
	})

})
