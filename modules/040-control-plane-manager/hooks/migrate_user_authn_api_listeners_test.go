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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// The ListenerSet of user-authn 1.76 with the API published over HTTPS.
const userAuthnListenerSetWithHTTPSAPI = `
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: user-authn
  namespace: d8-user-authn
spec:
  listeners:
  - name: dex
    hostname: dex.example.com
    port: 443
    protocol: HTTPS
  - name: dex-redirect
    hostname: dex.example.com
    port: 80
    protocol: HTTP
  - name: api
    hostname: api.example.com
    port: 443
    protocol: HTTPS
  - name: api-redirect
    hostname: api.example.com
    port: 80
    protocol: HTTP
  - name: kubeconfig
    hostname: kubeconfig.example.com
    port: 443
    protocol: HTTPS
  - name: kubeconfig-redirect
    hostname: kubeconfig.example.com
    port: 80
    protocol: HTTP
  parentRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: shared-gateway
    namespace: d8-alb
`

// The ListenerSet of user-authn 1.76 with the API published over plain HTTP.
const userAuthnListenerSetWithHTTPAPI = `
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: user-authn
  namespace: d8-user-authn
spec:
  listeners:
  - name: dex
    hostname: dex.example.com
    port: 80
    protocol: HTTP
  - name: api
    hostname: api.example.com
    port: 80
    protocol: HTTP
  parentRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: shared-gateway
    namespace: d8-alb
`

// The ListenerSet of user-authn 1.77, which no longer publishes the API.
const userAuthnListenerSetWithoutAPI = `
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: user-authn
  namespace: d8-user-authn
spec:
  listeners:
  - name: dex
    hostname: dex.example.com
    port: 443
    protocol: HTTPS
  - name: dex-redirect
    hostname: dex.example.com
    port: 80
    protocol: HTTP
  parentRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: shared-gateway
    namespace: d8-alb
`

var _ = Describe("Modules :: control-plane-manager :: hooks :: migrate user-authn API listeners ::", func() {
	listenerNames := func(f *HookExecutionConfig) []string {
		listenerSet := f.KubernetesResource("ListenerSet", "d8-user-authn", "user-authn")
		Expect(listenerSet.Exists()).To(BeTrue())

		var names []string
		for _, name := range listenerSet.Field("spec.listeners.#.name").Array() {
			names = append(names, name.String())
		}
		return names
	}

	// The hook reads and annotates the Namespaces through the typed client, which the fake cluster keeps
	// apart from the objects of KubeStateSet.
	createNamespace := func(name string, annotations map[string]string) {
		_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	migrationRecorded := func() bool {
		kubeSystem, err := dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Get(context.TODO(), "kube-system", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return kubeSystem.Annotations["control-plane-manager.deckhouse.io/user-authn-api-listeners-migrated"] == "true"
	}

	Context("With the Gateway API in the cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{}}}`, `{}`)
		f.RegisterCRD("gateway.networking.k8s.io", "v1", "ListenerSet", true)

		Context("With the API published over HTTPS by user-authn", func() {
			BeforeEach(func() {
				f.KubeStateSet(userAuthnListenerSetWithHTTPSAPI)
				createNamespace("kube-system", nil)
				createNamespace("d8-user-authn", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should remove only the API listeners and record the migration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(listenerNames(f)).To(Equal([]string{"dex", "dex-redirect", "kubeconfig", "kubeconfig-redirect"}))
				Expect(migrationRecorded()).To(BeTrue())
			})
		})

		Context("With the API published over HTTP by user-authn", func() {
			BeforeEach(func() {
				f.KubeStateSet(userAuthnListenerSetWithHTTPAPI)
				createNamespace("kube-system", nil)
				createNamespace("d8-user-authn", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should remove the API listener and record the migration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(listenerNames(f)).To(Equal([]string{"dex"}))
				Expect(migrationRecorded()).To(BeTrue())
			})
		})

		Context("With a user-authn ListenerSet that has no API listeners", func() {
			BeforeEach(func() {
				f.KubeStateSet(userAuthnListenerSetWithoutAPI)
				createNamespace("kube-system", nil)
				createNamespace("d8-user-authn", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should keep the ListenerSet as it is and record the migration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(listenerNames(f)).To(Equal([]string{"dex", "dex-redirect"}))
				Expect(migrationRecorded()).To(BeTrue())
			})
		})

		Context("Without the user-authn ListenerSet", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				createNamespace("kube-system", nil)
				createNamespace("d8-user-authn", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should record the migration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(migrationRecorded()).To(BeTrue())
			})
		})

		Context("With the migration already recorded", func() {
			BeforeEach(func() {
				f.KubeStateSet(userAuthnListenerSetWithHTTPSAPI)
				createNamespace("kube-system", map[string]string{"control-plane-manager.deckhouse.io/user-authn-api-listeners-migrated": "true"})
				createNamespace("d8-user-authn", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should not touch the ListenerSet", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(listenerNames(f)).To(Equal([]string{"dex", "dex-redirect", "api", "api-redirect", "kubeconfig", "kubeconfig-redirect"}))
			})
		})

		Context("Without the user-authn Namespace", func() {
			BeforeEach(func() {
				f.KubeStateSet(userAuthnListenerSetWithHTTPSAPI)
				createNamespace("kube-system", nil)
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should skip the migration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(listenerNames(f)).To(Equal([]string{"dex", "dex-redirect", "api", "api-redirect", "kubeconfig", "kubeconfig-redirect"}))
				Expect(migrationRecorded()).To(BeFalse())
			})
		})
	})

	Context("Without the Gateway API in the cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{}}}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			createNamespace("kube-system", nil)
			createNamespace("d8-user-authn", nil)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should record the migration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(migrationRecorded()).To(BeTrue())
		})
	})
})
