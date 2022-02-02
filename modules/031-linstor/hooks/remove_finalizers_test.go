/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: linstor :: hooks :: remove-finalizers ::", func() {

	Context("Finalizers are exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorController", true)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorSatelliteSet", true)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorCSIDriver", true)

		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorController
metadata:
  name: linstor
  namespace: d8-linstor
  finalizers:
  - finalizer.linstor-controller.linbit.com
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorSatelliteSet
metadata:
  name: linstor
  namespace: d8-linstor
  finalizers:
  - finalizer.linstor-controller.linbit.com
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorCSIDriver
metadata:
  name: linstor
  namespace: d8-linstor
  finalizers:
  - finalizer.linstor-controller.linbit.com
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Expected patch", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(3))
		})
		It("All finalizers should be removed", func() {
			cr := f.KubernetesResource("LinstorController", "d8-linstor", "linstor")
			Expect(cr.Field(`metadata.finalizers`).Exists()).To(BeFalse())
			cr = f.KubernetesResource("LinstorSatelliteSet", "d8-linstor", "linstor")
			Expect(cr.Field(`metadata.finalizers`).Exists()).To(BeFalse())
			cr = f.KubernetesResource("LinstorCSIDriver", "d8-linstor", "linstor")
			Expect(cr.Field(`metadata.finalizers`).Exists()).To(BeFalse())
		})
	})

	Context("Finalizers are not exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorController", true)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorSatelliteSet", true)
		f.RegisterCRD("piraeus.linbit.com", "v1", "LinstorCSIDriver", true)

		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorController
metadata:
  name: linstor
  namespace: d8-linstor
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorSatelliteSet
metadata:
  name: linstor
  namespace: d8-linstor
---
apiVersion: piraeus.linbit.com/v1
kind: LinstorCSIDriver
metadata:
  name: linstor
  namespace: d8-linstor
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Patch is not expected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
	})

})
