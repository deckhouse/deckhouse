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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: safe_daemonset_update ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.1", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", true)

	dsControllerMainInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
    ingress-nginx-safe-update: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: controller
      name: main
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
`
	dsProxyMainFailoverInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: proxy-main-failover
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: proxy-failover
    ingress-nginx-safe-update: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: proxy-failover
      name: main
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
`
	dsControllerMainFailoverInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main-failover
  namespace: d8-ingress-nginx
  labels:
    name: main-failover
    app: controller
    ingress-nginx-failover: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: controller
      name: main-failover
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
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
    pod-template-generation: "1"
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
    pod-template-generation: "1"
`
	pod1ProxyMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-main-failover-1
  namespace: d8-ingress-nginx
  labels:
    app: proxy-failover
    name: main
    pod-template-generation: "1"
`
	pod2ProxyMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-main-failover-2
  namespace: d8-ingress-nginx
  labels:
    app: proxy-failover
    name: main
    pod-template-generation: "1"
`
	pod1ControllerMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-failover-1
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main-failover
    pod-template-generation: "1"
`
	pod2ControllerMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-failover-2
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main-failover
    pod-template-generation: "1"
`
	var dsControllerMain *v1.DaemonSet
	var dsProxyMainFailover *v1.DaemonSet
	var dsControllerMainFailover *v1.DaemonSet
	var pod1ControllerMain *corev1.Pod
	var pod2ControllerMain *corev1.Pod
	var pod1ProxyMainFailover *corev1.Pod
	var pod2ProxyMainFailover *corev1.Pod
	var pod1ControllerMainFailover *corev1.Pod
	var pod2ControllerMainFailover *corev1.Pod

	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(dsControllerMainInitialYAML), &dsControllerMain)
		_ = yaml.Unmarshal([]byte(dsProxyMainFailoverInitialYAML), &dsProxyMainFailover)
		_ = yaml.Unmarshal([]byte(dsControllerMainFailoverInitialYAML), &dsControllerMainFailover)
		_ = yaml.Unmarshal([]byte(pod1ControllerMainInitialYAML), &pod1ControllerMain)
		_ = yaml.Unmarshal([]byte(pod2ControllerMainInitialYAML), &pod2ControllerMain)
		_ = yaml.Unmarshal([]byte(pod1ProxyMainFailoverInitialYAML), &pod1ProxyMainFailover)
		_ = yaml.Unmarshal([]byte(pod2ProxyMainFailoverInitialYAML), &pod2ProxyMainFailover)
		_ = yaml.Unmarshal([]byte(pod1ControllerMainFailoverInitialYAML), &pod1ControllerMainFailover)
		_ = yaml.Unmarshal([]byte(pod2ControllerMainFailoverInitialYAML), &pod2ControllerMainFailover)
	})

	Context("all daemonsets updated", func() {
		BeforeEach(func() {
			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("must be execute successfully without any changes", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset controller-main update scheduled", func() {
		BeforeEach(func() {
			dsControllerMain.Generation = 2
			dsControllerMain.Status.UpdatedNumberScheduled = 1
			pod2ControllerMain.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod controller-main-1 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).ToNot(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset controller-main update scheduled", func() {
		BeforeEach(func() {
			dsControllerMain.Generation = 2
			dsControllerMain.Status.UpdatedNumberScheduled = 1
			pod1ControllerMain.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")
			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod controller-main-2 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			_, pod2ControllerMainError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod2ControllerMainError)).To(BeTrue())
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset proxy-main-failover update scheduled", func() {
		BeforeEach(func() {
			dsProxyMainFailover.Generation = 2
			dsProxyMainFailover.Status.UpdatedNumberScheduled = 1
			pod2ProxyMainFailover.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod proxy-main-failover-1 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			_, pod1ProxyMainFailoverError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod1ProxyMainFailoverError)).To(BeTrue())
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset proxy-main-failover update scheduled", func() {
		BeforeEach(func() {
			dsProxyMainFailover.Generation = 2
			dsProxyMainFailover.Status.UpdatedNumberScheduled = 1
			pod1ProxyMainFailover.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod proxy-main-failover-2 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			_, pod2ProxyMainFailoverError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod2ProxyMainFailoverError)).To(BeTrue())
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("all pods with CrashLoopBackOff status", func() {
		BeforeEach(func() {
			containerStatusCrashLoopBackOff := corev1.ContainerStatus{Name: "controller", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}
			pod1ControllerMain.Status.ContainerStatuses = append(pod1ControllerMain.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod2ControllerMain.Status.ContainerStatuses = append(pod2ControllerMain.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod1ProxyMainFailover.Status.ContainerStatuses = append(pod1ProxyMainFailover.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod2ProxyMainFailover.Status.ContainerStatuses = append(pod2ProxyMainFailover.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			dsControllerMain.Generation = 2
			dsProxyMainFailover.Generation = 2

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})
			f.RunHook()
		})

		It("all pods must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).ToNot(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).ToNot(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).ToNot(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).ToNot(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})
})
