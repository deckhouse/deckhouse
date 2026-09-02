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

package webhook

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/testenv"
)

func envYandexInstanceClass(name string, consumers []string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "YandexInstanceClass"})
	u.SetName(name)
	u.Object["spec"] = map[string]any{"cores": int64(2), "memory": int64(4096)}
	if len(consumers) > 0 {
		groups := make([]any, 0, len(consumers))
		for _, c := range consumers {
			groups = append(groups, c)
		}
		u.Object["status"] = map[string]any{"nodeGroupConsumers": groups}
	}
	return u
}

func cleanupObject(obj client.Object) {
	err := k8sClient.Delete(suiteCtx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		// InstanceClass deletion is blocked while nodeGroupConsumers is set; clear it first.
		got := obj.DeepCopyObject().(client.Object)
		if getErr := k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(obj), got); getErr == nil {
			u, ok := got.(*unstructured.Unstructured)
			if ok {
				unstructured.RemoveNestedField(u.Object, "status")
				Expect(k8sClient.Update(suiteCtx, u)).To(Succeed())
			}
		}
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, obj))).To(Succeed())
	}
}

// User story: As a cluster administrator, I want conflicting NodeUsers, duplicate
// StaticInstance addresses and deletion of in-use InstanceClasses rejected by the
// apiserver itself, so that invalid node-management state can never be persisted.
var _ = Describe("migrated validating webhooks", func() {
	BeforeEach(func() {
		apiWarnings.reset()
	})

	Context("NodeUser", func() {
		It("rejects a duplicate uid in an overlapping nodeGroup", func() {
			existing := nodeUserObject(testenv.UniqueName("nu"), 1101, []string{"worker"}, "hash")
			Expect(k8sClient.Create(suiteCtx, existing)).To(Succeed())
			DeferCleanup(cleanupObject, existing)

			dup := nodeUserObject(testenv.UniqueName("nu"), 1101, []string{"worker"}, "hash")
			err := k8sClient.Create(suiteCtx, dup)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("The user with the uid: 1101 already exists in the nodeGroup: worker"))
		})

		It("allows the same uid in disjoint nodeGroups", func() {
			existing := nodeUserObject(testenv.UniqueName("nu"), 1102, []string{"front"}, "hash")
			Expect(k8sClient.Create(suiteCtx, existing)).To(Succeed())
			DeferCleanup(cleanupObject, existing)

			other := nodeUserObject(testenv.UniqueName("nu"), 1102, []string{"worker"}, "hash")
			Expect(k8sClient.Create(suiteCtx, other)).To(Succeed())
			DeferCleanup(cleanupObject, other)
		})

		It("rejects a new user when an existing same-uid user applies to all nodeGroups", func() {
			existing := nodeUserObject(testenv.UniqueName("nu"), 1103, []string{"*"}, "hash")
			Expect(k8sClient.Create(suiteCtx, existing)).To(Succeed())
			DeferCleanup(cleanupObject, existing)

			dup := nodeUserObject(testenv.UniqueName("nu"), 1103, []string{"worker"}, "hash")
			err := k8sClient.Create(suiteCtx, dup)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`The user with the uid: 1103 already exists in the nodeGroup: "*"`))
		})

		It("allows updating a user without conflicting with itself", func() {
			user := nodeUserObject(testenv.UniqueName("nu"), 1104, []string{"worker"}, "hash")
			Expect(k8sClient.Create(suiteCtx, user)).To(Succeed())
			DeferCleanup(cleanupObject, user)

			Expect(unstructured.SetNestedField(user.Object, "new-hash", "spec", "passwordHash")).To(Succeed())
			Expect(k8sClient.Update(suiteCtx, user)).To(Succeed())
		})

		It("warns when passwordHash is empty", func() {
			user := nodeUserObject(testenv.UniqueName("nu"), 1105, []string{"worker"}, "")
			Expect(k8sClient.Create(suiteCtx, user)).To(Succeed())
			DeferCleanup(cleanupObject, user)

			Expect(apiWarnings.all()).To(ContainElement(
				"Password hash is empty. This may not be secure and it may be prohibited by PAM settings."))
		})
	})

	Context("StaticInstance", func() {
		It("rejects a StaticInstance reusing an occupied address", func() {
			existing := staticInstanceObject(testenv.UniqueName("si"), "10.10.0.1")
			Expect(k8sClient.Create(suiteCtx, existing)).To(Succeed())
			DeferCleanup(cleanupObject, existing)

			dupName := testenv.UniqueName("si")
			dup := staticInstanceObject(dupName, "10.10.0.1")
			err := k8sClient.Create(suiteCtx, dup)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"staticinstances.deckhouse.io %q, static instance %q is already using the address %q",
				dupName, existing.GetName(), "10.10.0.1")))
		})

		It("allows a StaticInstance with a unique address", func() {
			first := staticInstanceObject(testenv.UniqueName("si"), "10.10.0.2")
			Expect(k8sClient.Create(suiteCtx, first)).To(Succeed())
			DeferCleanup(cleanupObject, first)

			second := staticInstanceObject(testenv.UniqueName("si"), "10.10.0.3")
			Expect(k8sClient.Create(suiteCtx, second)).To(Succeed())
			DeferCleanup(cleanupObject, second)
		})

		It("allows updating a StaticInstance keeping its own address", func() {
			instance := staticInstanceObject(testenv.UniqueName("si"), "10.10.0.4")
			Expect(k8sClient.Create(suiteCtx, instance)).To(Succeed())
			DeferCleanup(cleanupObject, instance)

			instance.SetLabels(map[string]string{"test": "label"})
			Expect(k8sClient.Update(suiteCtx, instance)).To(Succeed())
		})
	})

	Context("InstanceClass", func() {
		It("rejects deleting an InstanceClass that NodeGroups still use", func() {
			name := testenv.UniqueName("ic")
			ic := envYandexInstanceClass(name, []string{"ng1", "ng2"})
			Expect(k8sClient.Create(suiteCtx, ic)).To(Succeed())
			DeferCleanup(cleanupObject, ic)

			err := k8sClient.Delete(suiteCtx, ic)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"YandexInstanceClass/%s cannot be deleted because it is being used by NodeGroup: ng1, ng2", name)))
		})

		It("allows deleting an InstanceClass without consumers", func() {
			ic := envYandexInstanceClass(testenv.UniqueName("ic"), nil)
			Expect(k8sClient.Create(suiteCtx, ic)).To(Succeed())
			Expect(k8sClient.Delete(suiteCtx, ic)).To(Succeed())
		})

		It("allows deleting an InstanceClass after its consumers are gone", func() {
			ic := envYandexInstanceClass(testenv.UniqueName("ic"), []string{"ng1"})
			Expect(k8sClient.Create(suiteCtx, ic)).To(Succeed())

			Expect(k8sClient.Delete(suiteCtx, ic)).NotTo(Succeed())

			Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(ic), ic)).To(Succeed())
			unstructured.RemoveNestedField(ic.Object, "status")
			Expect(k8sClient.Update(suiteCtx, ic)).To(Succeed())
			Expect(k8sClient.Delete(suiteCtx, ic)).To(Succeed())
		})
	})
})
