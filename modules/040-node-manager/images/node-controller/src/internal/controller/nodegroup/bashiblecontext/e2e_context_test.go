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

package bashiblecontext

import (
	"net"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a node joining the cluster, I want the bashible context to carry the master
// addresses that are current right now, so that I bootstrap against a live apiserver instead of
// one that was replaced while the controller was not looking.
var _ = Describe("Bashible context endpoint discovery", Ordered, func() {
	apiserverEndpoints := func(g Gomega) []string {
		secret := &corev1.Secret{}
		g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: secretNamespace, Name: secretName,
		}, secret)).To(Succeed())

		var input map[string]interface{}
		g.Expect(yaml.Unmarshal(secret.Data[secretInputKey], &input)).To(Succeed())

		raw, _ := input["apiserverEndpoints"].([]interface{})
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if ep, ok := item.(string); ok {
				out = append(out, ep)
			}
		}
		return out
	}

	// The envtest apiserver publishes and keeps reconciling this EndpointSlice itself, so the
	// suite reads the live object instead of dictating its contents.
	liveEndpointSlice := func() *discoveryv1.EndpointSlice {
		slice := &discoveryv1.EndpointSlice{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: kubernetesEndpointSliceNS, Name: kubernetesEndpointSliceName,
		}, slice)).To(Succeed())
		return slice
	}

	httpsPort := func(slice *discoveryv1.EndpointSlice) int32 {
		for _, port := range slice.Ports {
			if port.Name != nil && *port.Name == "https" && port.Port != nil {
				return *port.Port
			}
		}
		Fail("kubernetes EndpointSlice has no https port")
		return 0
	}

	It("publishes the addresses of the kubernetes EndpointSlice", func() {
		slice := liveEndpointSlice()
		port := httpsPort(slice)
		expected := make([]string, 0, len(slice.Endpoints))
		for _, endpoint := range slice.Endpoints {
			for _, addr := range endpoint.Addresses {
				expected = append(expected, net.JoinHostPort(addr, strconv.Itoa(int(port))))
			}
		}
		Expect(expected).NotTo(BeEmpty(), "the apiserver must publish its own endpoint")

		Eventually(func(g Gomega) []string {
			return apiserverEndpoints(g)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(ContainElements(expected))
	})

	It("picks up an EndpointSlice change without waiting for the resync", func() {
		slice := liveEndpointSlice()
		port := httpsPort(slice)
		added := net.JoinHostPort("10.10.0.99", strconv.Itoa(int(port)))

		patch := client.MergeFrom(slice.DeepCopy())
		slice.Endpoints = append(slice.Endpoints, discoveryv1.Endpoint{Addresses: []string{"10.10.0.99"}})
		Expect(k8sClient.Patch(suiteCtx, slice, patch)).To(Succeed())

		// The apiserver's own endpoint reconciler drops the extra address again after its
		// period, so this asserts the published context follows the object within seconds. The
		// resync is ten minutes: without the EndpointSlice watch, nothing arrives in time.
		Eventually(func(g Gomega) []string {
			return apiserverEndpoints(g)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(ContainElement(added))
	})

	It("discovers kube-apiserver pods through the scoped cache", func() {
		// The Pod read is served by the informer scoped to exactly these labels. If Pod reads
		// were routed around the cache, or the scope stopped matching, this never shows up.
		pod := &corev1.Pod{}
		pod.Namespace = kubeSystemNS
		pod.Name = testenv.UniqueName("kube-apiserver")
		pod.Labels = map[string]string{"component": "kube-apiserver", "tier": "control-plane"}
		pod.Spec = corev1.PodSpec{Containers: []corev1.Container{{
			Name:  "kube-apiserver",
			Image: "registry.k8s.io/kube-apiserver:v1.31.0",
		}}}
		Expect(k8sClient.Create(suiteCtx, pod)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, pod))).To(Succeed())
		})

		pod.Status.PodIP = "10.20.0.7"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(suiteCtx, pod)).To(Succeed())

		Eventually(func(g Gomega) []string {
			return apiserverEndpoints(g)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(ContainElement("10.20.0.7:6443"))
	})
})
