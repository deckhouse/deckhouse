/*
Copyright 2025 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: cleanup cni daemonset hostports ::", func() {
	const (
		initValuesString            = `{"istio": {"internal": {}}}`
		initConfigValuesString      = `{}`
		dsHostNetworkFalseWithPorts = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: istio-cni-node
  namespace: d8-istio
spec:
  template:
    spec:
      hostNetwork: false
      containers:
      - name: install-cni
        command:
        - install-cni
        args:
        - --log_output_level=default:info
        securityContext:
          privileged: true
          runAsGroup: 0
          runAsNonRoot: false
          runAsUser: 0
        ports:
        - containerPort: 8000
          hostPort: 8000
          protocol: TCP
      - name: kube-rbac-proxy
        ports:
        - containerPort: 9734
          hostPort: 4286
          protocol: TCP
`
		dsHostNetworkFalseWithoutPorts = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: istio-cni-node
  namespace: d8-istio
spec:
  template:
    spec:
      hostNetwork: false
      containers:
      - name: install-cni
        command:
        - install-cni
        args:
        - --log_output_level=default:info
        securityContext:
          privileged: true
          runAsGroup: 0
          runAsNonRoot: false
          runAsUser: 0
        ports:
        - containerPort: 8000
          protocol: TCP
      - name: kube-rbac-proxy
        ports:
        - containerPort: 9734
          protocol: TCP
`
		dsHostNetworkTrue = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: istio-cni-node
  namespace: d8-istio
spec:
  template:
    spec:
      hostNetwork: true
      containers:
      - name: install-cni
        command:
        - install-cni
        args:
        - --log_output_level=default:info
        securityContext:
          privileged: true
          runAsGroup: 0
          runAsNonRoot: false
          runAsUser: 0
        ports:
        - containerPort: 8000
          hostPort: 8000
          protocol: TCP
      - name: kube-rbac-proxy
        ports:
        - containerPort: 9734
          hostPort: 4286
          protocol: TCP
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Without DaemonSet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Get(context.TODO(), "istio-cni-node", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("With hostNetwork=false (null) and hostPorts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsHostNetworkFalseWithPorts))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsHostNetworkFalseWithPorts), &ds)
			_, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.RunHook()
		})

		It("Should remove all hostPorts", func() {
			Expect(f).To(ExecuteSuccessfully())
			ds, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Get(context.TODO(), "istio-cni-node", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeFalse())
			for _, c := range ds.Spec.Template.Spec.Containers {
				for _, p := range c.Ports {
					Expect(p.HostPort).To(BeZero())
				}
			}
		})
	})

	Context("With hostNetwork=false (null) and without hostPorts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsHostNetworkFalseWithoutPorts))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsHostNetworkFalseWithoutPorts), &ds)
			_, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.RunHook()
		})

		It("Should not modify the DaemonSet", func() {
			Expect(f).To(ExecuteSuccessfully())
			ds, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Get(context.TODO(), "istio-cni-node", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeFalse())
			for _, c := range ds.Spec.Template.Spec.Containers {
				for _, p := range c.Ports {
					Expect(p.HostPort).To(BeZero())
				}
			}
		})
	})

	Context("With hostNetwork=true", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsHostNetworkTrue))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsHostNetworkTrue), &ds)
			_, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.RunHook()
		})

		It("Should preserve hostPorts", func() {
			Expect(f).To(ExecuteSuccessfully())
			ds, err := dependency.TestDC.K8sClient.AppsV1().DaemonSets("d8-istio").Get(context.TODO(), "istio-cni-node", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(8000)))
			Expect(ds.Spec.Template.Spec.Containers[1].Ports[0].HostPort).To(Equal(int32(4286)))
		})
	})
})
