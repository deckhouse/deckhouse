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

package capi

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The enumeration and the lazy source live in internal/common, but they mean nothing without a
// real apiserver and a provider CRD — both of which this suite already carries. The lazy source
// is driven directly against its own cache and queue; the handler encodes the GVK version into
// the request name so events arriving through different informers stay distinguishable.
var _ = Describe("InstanceClass registration enumeration and lazy watches", func() {
	dvpAlpha := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DVPInstanceClass"}

	newRegistrationSecret := func(name, kind, version string) *corev1.Secret {
		secret := &corev1.Secret{}
		secret.Namespace = cloudprovider.SecretNamespace
		secret.Name = name
		secret.Labels = map[string]string{cloudprovider.SecretLabel: ""}
		secret.Data = map[string][]byte{
			cloudprovider.InstanceClassKindKey:       []byte(kind),
			cloudprovider.InstanceClassAPIVersionKey: []byte(version),
		}
		return secret
	}

	It("enumerates the suite's registration through the label", func() {
		registry, err := cloudprovider.Load(suiteCtx, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		gvks := registry.InstanceClassGVKs()
		Expect(gvks).To(Equal([]schema.GroupVersionKind{dvpAlpha}))
	})

	It("collapses the legacy and the per-provider Secret into one GVK", func() {
		double := newRegistrationSecret("d8-node-manager-cloud-provider-dvp", "DVPInstanceClass", "v1alpha1")
		Expect(k8sClient.Create(suiteCtx, double)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, double) })

		registry, err := cloudprovider.Load(suiteCtx, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		gvks := registry.InstanceClassGVKs()
		Expect(gvks).To(Equal([]schema.GroupVersionKind{dvpAlpha}))
	})

	It("ignores a registration that does not publish the version", func() {
		unversioned := newRegistrationSecret("d8-node-manager-cloud-provider-aws", "AWSInstanceClass", "")
		Expect(k8sClient.Create(suiteCtx, unversioned)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, unversioned) })

		registry, err := cloudprovider.Load(suiteCtx, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		gvks := registry.InstanceClassGVKs()
		Expect(gvks).To(Equal([]schema.GroupVersionKind{dvpAlpha}),
			"a kind without a version must contribute nothing — guessing a version is what this mechanism prevents")
	})

	It("watches the registered kind immediately, and a kind registered later without any restart", func() {
		ctx, cancel := context.WithCancel(suiteCtx)
		DeferCleanup(cancel)

		By("running the lazy source against its own cache and queue")
		sourceCache, err := cache.New(cfg, cache.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(sourceCache.Start(ctx)).To(Succeed())
		}()
		Expect(sourceCache.WaitForCacheSync(ctx)).To(BeTrue())

		queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		versionedName := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name: obj.GetObjectKind().GroupVersionKind().Version + "-" + obj.GetName(),
			}}}
		})
		Expect(cloudprovider.LazyInstanceClassSource(sourceCache, versionedName).Start(ctx, queue)).To(Succeed())

		drainUntil := func(want string) {
			GinkgoHelper()
			Eventually(func() string {
				if queue.Len() == 0 {
					return ""
				}
				req, _ := queue.Get()
				queue.Done(req)
				return req.Name
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Equal(want))
		}

		By("the suite's registration is picked up from the store replay, no event needed")
		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName(testenv.UniqueName("lazy"))
		Expect(unstructured.SetNestedField(ic.Object, "test", "spec", "vmClassName")).To(Succeed())
		Expect(k8sClient.Create(ctx, ic)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ic) })
		drainUntil("v1alpha1-" + ic.GetName())

		By("registering the same kind at another served version after the source started")
		late := newRegistrationSecret("d8-node-manager-cloud-provider-late", "DVPInstanceClass", "v1")
		Expect(k8sClient.Create(ctx, late)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, late) })

		By("an edit of the object now arrives through the new version's informer as well")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ic.GetName()}, ic)).To(Succeed())
			g.Expect(unstructured.SetNestedField(ic.Object, testenv.UniqueName("edit"), "spec", "vmClassName")).To(Succeed())
			g.Expect(k8sClient.Update(ctx, ic)).To(Succeed())

			seen := map[string]bool{}
			for queue.Len() > 0 {
				req, _ := queue.Get()
				queue.Done(req)
				seen[req.Name] = true
			}
			g.Expect(seen).To(HaveKey("v1-"+ic.GetName()),
				"the late registration must have started a watch at its version, with nothing restarted")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})
})
