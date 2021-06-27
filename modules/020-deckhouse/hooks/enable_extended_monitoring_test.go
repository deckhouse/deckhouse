/*
Copyright 2021 Flant CJSC

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

var _ = Describe("Deckhouse hooks :: enable_extended_monitoring ::", func() {
	const (
		kubeSystemAndD8SystemPresent = `
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
`
	)

	f := HookExecutionConfigInit("", "")

	Context("kube-system and d8-system present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeSystemAndD8SystemPresent))
			f.RunHook()
		})

		It("Annotations should be present on both namespaces", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("namespace", "", "d8-system-system").Field("metadata.annotations.extended-monitoring.flant.com/enabled").String()).
				To(Equal(""))
			Expect(f.KubernetesResource("namespace", "", "kube-system").Field("metadata.annotations.extended-monitoring.flant.com/enabled").String()).
				To(Equal(""))
		})
	})
})
