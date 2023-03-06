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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	ktest "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// TODO: add actual Eviction behavior once a real apiserver is introduced into tests

var _ = Describe("ingress-nginx :: hooks :: chaos_monkey ::", func() {
	ingressNginxControllerMainInitialYAML := `
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  chaosMonkey: true
`

	daemonsetControllerMainInitialYAML := func(ready bool) string {
		readyNumber := 2
		if ready {
			readyNumber = 3
		}
		return fmt.Sprintf(`
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
spec:
  selector:
    matchLabels:
      app: controller
      name: main
status:
  desiredNumberScheduled: 3
  numberReady: %d
`, readyNumber)
	}

	pod1ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-11-01T07:23:55Z"
  name: controller-main-1
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
spec:
  nodeName: test1
`
	pod2ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-11-01T07:23:56Z"
  name: controller-main-2
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
spec:
  nodeName: test2
`
	pod3ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-11-01T07:23:57Z"
  name: controller-main-3
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
spec:
  nodeName: test2
`

	f := HookExecutionConfigInit("", "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressNginxController", true)

	// only to initialize fake cluster
	f.KubeStateSet(``)

	Context("with not ready DaemonSet", func() {
		var createdEviction *policyv1.Eviction

		BeforeEach(func() {
			f.KubeStateSet(ingressNginxControllerMainInitialYAML + daemonsetControllerMainInitialYAML(false))

			createPod(f.KubeClient(), pod1ControllerMainInitialYAML)
			createPod(f.KubeClient(), pod2ControllerMainInitialYAML)
			createPod(f.KubeClient(), pod3ControllerMainInitialYAML)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must execute successfully and does not evict anything", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(BeNil())
		})
	})

	Context("with ready DaemonSet", func() {
		var createdEviction *policyv1.Eviction

		BeforeEach(func() {
			f.KubeStateSet(ingressNginxControllerMainInitialYAML + daemonsetControllerMainInitialYAML(true))

			createPod(f.KubeClient(), pod1ControllerMainInitialYAML)
			createPod(f.KubeClient(), pod2ControllerMainInitialYAML)
			createPod(f.KubeClient(), pod3ControllerMainInitialYAML)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must evict the first pod (the oldest)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(Not(BeNil()))
			Expect(createdEviction.Name).To(Equal("controller-main-1"))
		})
	})

	Context("with ready DaemonSet with one pod", func() {
		var createdEviction *policyv1.Eviction

		BeforeEach(func() {
			f.KubeStateSet(ingressNginxControllerMainInitialYAML + daemonsetControllerMainInitialYAML(true))

			createPod(f.KubeClient(), pod1ControllerMainInitialYAML)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must skip", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(BeNil())
		})
	})
})

func createPod(kubeClient client.KubeClient, spec string) {
	var pod corev1.Pod
	if err := yaml.Unmarshal([]byte(spec), &pod); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), &pod, metav1.CreateOptions{})
}

func registerEvictionReactor(evictionObjPtr **policyv1.Eviction) {
	dependency.TestDC.K8sClient.CoreV1().(*fake.FakeCoreV1).PrependReactor("create", "pods", func(action ktest.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "eviction" {
			return false, nil, nil
		}

		*evictionObjPtr = action.(ktest.CreateAction).GetObject().(*policyv1.Eviction)

		return true, nil, nil
	})
}
