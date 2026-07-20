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
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: handle_virtual_control_plane ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`, `{}`)

	virtualControlPlaneResource := schema.GroupVersionResource{
		Group:    "control-plane.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "virtualcontrolplanes",
	}
	f.RegisterCRD(virtualControlPlaneResource.Group, virtualControlPlaneResource.Version, "VirtualControlPlane", false)

	const virtualControlPlane = `
---
apiVersion: control-plane.deckhouse.io/v1alpha1
kind: VirtualControlPlane
metadata:
  name: tenant-a
spec:
  kubernetesVersion: "1.31"
`

	Context("There are no VirtualControlPlane resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("sets internal value to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(hasVirtualControlPlanePath).Bool()).To(BeFalse())
		})
	})

	Context("There is at least one VirtualControlPlane resource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(virtualControlPlane))
			f.RunHook()
		})

		It("sets internal value to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(hasVirtualControlPlanePath).Bool()).To(BeTrue())
		})

		Context("VirtualControlPlane resource was removed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("sets internal value back to false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(hasVirtualControlPlanePath).Bool()).To(BeFalse())
			})
		})
	})
})
