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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	ktest "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

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

	deploymentControllerMainInitialYAML := `
---
apiVersion: apps/v1
kind: Deployment
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
`

	pod1ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
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
  name: controller-main-3
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
spec:
  nodeName: test2
`

	var (
		ingressNginxController   = &unstructured.Unstructured{}
		deploymentControllerMain *appsv1.Deployment
		pod1ControllerMain       *corev1.Pod
		pod2ControllerMain       *corev1.Pod
		pod3ControllerMain       *corev1.Pod
	)

	_ = yaml.Unmarshal([]byte(deploymentControllerMainInitialYAML), &deploymentControllerMain)
	_ = yaml.Unmarshal([]byte(pod1ControllerMainInitialYAML), &pod1ControllerMain)
	_ = yaml.Unmarshal([]byte(pod2ControllerMainInitialYAML), &pod2ControllerMain)
	_ = yaml.Unmarshal([]byte(pod3ControllerMainInitialYAML), &pod3ControllerMain)

	var ingressNginxControllerMap map[string]interface{}
	_ = yaml.Unmarshal([]byte(ingressNginxControllerMainInitialYAML), &ingressNginxControllerMap)
	ingressNginxController.SetUnstructuredContent(ingressNginxControllerMap)

	f := HookExecutionConfigInit("", "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressNginxController", true)

	Context("empty cluster", func() {
		var createdEviction *policyv1beta1.Eviction

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNginxControllerMainInitialYAML + deploymentControllerMainInitialYAML))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(BeNil())
		})
	})

	Context("cluster with single Pod on each Node", func() {
		var createdEviction *policyv1beta1.Eviction

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNginxControllerMainInitialYAML + deploymentControllerMainInitialYAML))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			_, _ = f.KubeClient().CoreV1().Pods(pod1ControllerMain.Namespace).Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods(pod2ControllerMain.Namespace).Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(BeNil())
		})
	})

	Context("cluster with two Pods on one Node", func() {
		var createdEviction *policyv1beta1.Eviction

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNginxControllerMainInitialYAML + deploymentControllerMainInitialYAML))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			_, _ = f.KubeClient().CoreV1().Pods(pod2ControllerMain.Namespace).Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods(pod3ControllerMain.Namespace).Create(context.TODO(), pod3ControllerMain, metav1.CreateOptions{})

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(Not(BeNil()))
		})
	})

	Context("cluster with two Pods on one Node and one additional Pod on a separate Node", func() {
		var createdEviction *policyv1beta1.Eviction

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNginxControllerMainInitialYAML + deploymentControllerMainInitialYAML))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			_, _ = f.KubeClient().CoreV1().Pods(pod1ControllerMain.Namespace).Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods(pod2ControllerMain.Namespace).Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods(pod3ControllerMain.Namespace).Create(context.TODO(), pod3ControllerMain, metav1.CreateOptions{})

			registerEvictionReactor(&createdEviction)

			f.RunHook()
		})

		It("must execute successfully, first Pod should not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createdEviction).To(Not(BeNil()))

			_, err := f.KubeClient().CoreV1().Pods(pod1ControllerMain.Namespace).Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			Expect(err).To(BeNil())
		})
	})
})

func registerEvictionReactor(evictionObjPtr **policyv1beta1.Eviction) {
	dependency.TestDC.K8sClient.CoreV1().(*fake.FakeCoreV1).PrependReactor("create", "pods", func(action ktest.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "eviction" {
			return false, nil, nil
		}

		*evictionObjPtr = action.(ktest.CreateAction).GetObject().(*policyv1beta1.Eviction)

		return true, nil, nil
	})
}
