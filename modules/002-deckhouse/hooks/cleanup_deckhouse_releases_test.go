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

// TODO: rm
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
		It("Wrong deployed releases should be Superseded", func() {
			Expect(f).To(ExecuteSuccessfully())
			rl1 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.28.0")
			Expect(rl1.Field("status.phase").String()).Should(Equal("Superseded"))
			rl2 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.28.1")
			Expect(rl2.Field("status.phase").String()).Should(Equal("Superseded"))
			rl3 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.28.2")
			Expect(rl3.Field("status.phase").String()).Should(Equal("Superseded"))
			rl4 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.28.3")
			Expect(rl4.Field("status.phase").String()).Should(Equal("Deployed"))
		})
	})

	Context("Have 15 Superseded Releases", func() {
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
			rl1 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.28.0")
			Expect(rl1.Field("status.phase").String()).Should(Equal("Deployed"))

			ll, _ := f.KubeClient().Dynamic().Resource(schema.GroupVersionResource{Resource: "deckhousereleases", Group: "deckhouse.io", Version: "v1alpha1"}).List(context.TODO(), v1.ListOptions{})
			Expect(ll.Items).Should(HaveLen(6))
		})
	})

	Context("Releases from real cluster", func() {
		rl := `
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.30.16
spec:
  requirements:
    k8s: 1.19.0
  version: v1.30.16
status:
  approved: true
  message: ""
  phase: Superseded
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.30.17
spec:
  applyAfter: "2022-03-22T16:39:01.017873947Z"
  requirements:
    k8s: 1.19.0
  version: v1.30.17
status:
  approved: false
  message: ""
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.30.18
spec:
  requirements:
    k8s: 1.19.0
  version: v1.30.18
status:
  approved: true
  message: ""
  phase: Deployed

---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.30.19
spec:
  applyAfter: "2022-03-24T12:20:02.146704455Z"
  requirements:
    k8s: 1.19.0
  version: v1.30.19
status:
  approved: true
  message: ""
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.30.9
spec:
  version: v1.30.9
status:
  approved: true
  message: ""
  phase: Superseded
`
		BeforeEach(func() {
			bc := f.KubeStateSetAndWaitForBindingContexts(rl, 1)
			f.BindingContexts.Set(bc)
			f.RunHook()
		})
		It("Should keep last release in order", func() {
			Expect(f).To(ExecuteSuccessfully())
			rl19 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.19")
			rl18 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.18")
			rl17 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.17")
			Expect(rl17.Field("status.phase").String()).Should(Equal("Skipped"))
			Expect(rl18.Field("status.phase").String()).Should(Equal("Deployed"))
			Expect(rl19.Field("status.phase").String()).Should(Equal("Pending"))
		})
	})

	Context("Has Deployed releases", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: "2023-02-22T11:22:01Z"
  generation: 1
  name: v1-43-8
  resourceVersion: "2035414797"
  uid: 31be4cdd-2f35-458c-afe6-a227ba9e4d32
spec:
  applyAfter: "2023-02-22T11:52:01.994220949Z"
  changelog:
    candi:
      fixes:
      - impact: Fix restarts containerd services on nodes.
        pull_request: https://github.com/deckhouse/deckhouse/pull/3929
        summary: Update of containerd to .
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.43.8
  requirements:
    ingressNginx: "1.1"
    k8s: 1.21.0
  version: v1.43.8
status:
  approved: false
  message: ""
  phase: Superseded
  transitionTime: "2023-06-15T21:20:00.254776566Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: true
kind: DeckhouseRelease
metadata:
  creationTimestamp: "2023-03-29T08:42:01Z"
  generation: 2
  name: v1-44-4
  resourceVersion: "2035413896"
  uid: f33a9ae9-140d-4e8e-bfcd-071e454ee5db
spec:
  changelog:
    log-shipper:
      fixes:
      - pull_request: https://github.com/deckhouse/deckhouse/pull/4222
        summary: Fix throttling alert labels.
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.44.4
  requirements:
    ingressNginx: "1.1"
    k8s: 1.21.0
    nodesMinimalOSVersionUbuntu: "18.04"
  version: v1.44.4
status:
  approved: true
  message: ""
  phase: Deployed
  transitionTime: "2023-06-15T21:19:30.267810792Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: true
kind: DeckhouseRelease
metadata:
  creationTimestamp: "2023-05-25T13:18:01Z"
  generation: 2
  name: v1-45-11
  resourceVersion: "2077388270"
  uid: bd114b8b-41c6-4089-b73c-9a5a9c10048a
spec:
  applyAfter: "2023-05-25T15:48:01.682158774Z"
  changelog:
    helm:
      fixes:
      - pull_request: https://github.com/deckhouse/deckhouse/pull/4751
        summary: Fix deprecated k8s resources metrics.
    ingress-nginx:
      fixes:
      - pull_request: https://github.com/deckhouse/deckhouse/pull/4734
        summary: Add protection for ingress-nginx-controller daemonset migration.
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.45.11
  requirements:
    ingressNginx: "1.1"
    k8s: 1.22.0
    nodesMinimalOSVersionUbuntu: "18.04"
  version: v1.45.11
status:
  approved: true
  message: ""
  phase: Superseded
  transitionTime: "2023-07-03T21:06:00.100969353Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: "2023-05-17T16:48:01Z"
  generation: 1
  name: v1-45-9
  resourceVersion: "2035447662"
  uid: 5ed4e921-c6ca-4085-ae1a-af0e47a1881c
spec:
  applyAfter: "2023-05-17T17:48:01.601411184Z"
  changelog:
    candi:
      fixes:
      - pull_request: https://github.com/deckhouse/deckhouse/pull/4669
        summary: Fix the error in the script.
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.45.9
  requirements:
    ingressNginx: "1.1"
    k8s: 1.22.0
    nodesMinimalOSVersionUbuntu: "18.04"
  version: v1.45.9
status:
  approved: false
  message: Skipped by cleanup hook
  phase: Skipped
  transitionTime: "2023-06-15T21:35:37.013703518Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: true
kind: DeckhouseRelease
metadata:
  creationTimestamp: "2023-06-23T07:24:01Z"
  generation: 2
  name: v1-46-10
  resourceVersion: "2077388269"
  uid: 4184d860-aa1b-457e-a11d-7d61199470eb
spec:
  applyAfter: "2023-06-23T08:24:01.700912496Z"
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.46.10
  requirements:
    ingressNginx: "1.1"
    k8s: 1.22.0
    nodesMinimalOSVersionUbuntu: "18.04"
  version: v1.46.10
status:
  approved: true
  message: ""
  phase: Deployed
  transitionTime: "2023-07-03T21:06:00.100966635Z"
`)
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should mark old deployed release as Superseded", func() {
			Expect(f).To(ExecuteSuccessfully())

			rel := f.KubernetesGlobalResource("DeckhouseRelease", "v1-44-4")
			Expect(rel.Field("status.phase").String()).Should(Equal("Superseded"))
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
  name: v1.28.%d
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
  name: v1.27.%d
spec:
  version: "v1.27.%d"
status:
  phase: Superseded
`, i, i)
		s.WriteString(rl)
	}

	return s.String()
}
