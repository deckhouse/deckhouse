/*
Copyright 2021 Flant JSC

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
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: cleanup deckhouse releases ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse":{}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DeckhouseRelease", false)

	Context("Have a few Deployed Releases", func() {
		BeforeEach(func() {
			state := generateReleases(4, 0)
			bc := f.KubeStateSetAndWaitForBindingContexts(state, 1)
			f.BindingContexts.Set(bc)
			f.RunHook()
		})
		It("Wrong deployed releases should be Outdated", func() {
			Expect(f).To(ExecuteSuccessfully())
			rl1 := f.KubernetesGlobalResource("DeckhouseRelease", "v1-28-0")
			Expect(rl1.Field("status.phase").String()).Should(Equal("Outdated"))
			rl2 := f.KubernetesGlobalResource("DeckhouseRelease", "v1-28-1")
			Expect(rl2.Field("status.phase").String()).Should(Equal("Outdated"))
			rl3 := f.KubernetesGlobalResource("DeckhouseRelease", "v1-28-2")
			Expect(rl3.Field("status.phase").String()).Should(Equal("Outdated"))
			rl4 := f.KubernetesGlobalResource("DeckhouseRelease", "v1-28-3")
			Expect(rl4.Field("status.phase").String()).Should(Equal("Deployed"))
		})
	})

	Context("Have 15 Outdated Releases", func() {
		BeforeEach(func() {
			state := generateReleases(0, 15)
			bc := f.KubeStateSetAndWaitForBindingContexts(state, 1)
			f.BindingContexts.Set(bc)
			f.RunHook()
		})
		It("Outdated releases (>10) should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			ll, _ := f.KubeClient().Dynamic().Resource(schema.GroupVersionResource{Resource: "deckhousereleases", Group: "deckhouse.io", Version: "v1alpha1"}).List(context.TODO(), v1.ListOptions{})
			Expect(ll.Items).Should(HaveLen(10))
		})
	})

	Context("Have 1 Deployed release and 5 Outdated Releases", func() {
		BeforeEach(func() {
			state := generateReleases(1, 5)
			bc := f.KubeStateSetAndWaitForBindingContexts(state, 1)
			f.BindingContexts.Set(bc)
			f.RunHook()
		})
		It("Shouldn't touch releases", func() {
			Expect(f).To(ExecuteSuccessfully())
			rl1 := f.KubernetesGlobalResource("DeckhouseRelease", "v1-28-0")
			Expect(rl1.Field("status.phase").String()).Should(Equal("Deployed"))

			ll, _ := f.KubeClient().Dynamic().Resource(schema.GroupVersionResource{Resource: "deckhousereleases", Group: "deckhouse.io", Version: "v1alpha1"}).List(context.TODO(), v1.ListOptions{})
			Expect(ll.Items).Should(HaveLen(6))
		})
	})
})

func generateReleases(deployedReleasesCount, outdatedReleasesCount int) string {
	s := strings.Builder{}

	for i := 0; i < deployedReleasesCount; i++ {
		rl := fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1-28-%d
spec:
  version: "v1.28.%d"
status:
  phase: Deployed
`, i, i)
		s.WriteString(rl)
	}

	for i := 0; i < outdatedReleasesCount; i++ {
		rl := fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1-27-%d
spec:
  version: "v1.27.%d"
status:
  phase: Outdated
`, i, i)
		s.WriteString(rl)
	}

	return s.String()
}
