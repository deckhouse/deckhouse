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

package crdmigration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var _ = Describe("MigrateSystemTypes", func() {
	It("rewrites the legacy spelling of NodeGroups and Node labels and leaves the rest alone", func(ctx SpecContext) {
		groups := []struct {
			name   string
			stored deckhousev1.SystemType
			want   deckhousev1.SystemType
		}{
			{name: "systemtype-immutable", stored: deckhousev1.SystemTypeLegacyImmutable, want: deckhousev1.SystemTypeDeckhouseEngine},
			{name: "systemtype-mutable", stored: deckhousev1.SystemTypeLegacyMutable, want: deckhousev1.SystemTypeGenericOS},
			{name: "systemtype-unset", stored: "", want: ""},
			{name: "systemtype-deckhouseengine", stored: deckhousev1.SystemTypeDeckhouseEngine, want: deckhousev1.SystemTypeDeckhouseEngine},
		}
		for _, group := range groups {
			ng := &deckhousev1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: group.name},
				Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic, SystemType: group.stored},
			}
			Expect(testenv.CreateNodeGroup(ctx, k8sClient, ng)).To(Succeed())
			DeferCleanup(func(ctx SpecContext) {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ng))).To(Succeed())
			})
		}

		createObject(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name: "systemtype-node-immutable",
			Labels: map[string]string{
				common.NodeSystemTypeLabel: string(deckhousev1.SystemTypeLegacyImmutable),
				common.NodeGroupLabel:      "worker",
			},
		}})
		createObject(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:   "systemtype-node-mutable",
			Labels: map[string]string{common.NodeSystemTypeLabel: string(deckhousev1.SystemTypeLegacyMutable)},
		}})
		createObject(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "systemtype-node-unlabelled"}})
		createObject(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:   "systemtype-node-deckhouseengine",
			Labels: map[string]string{common.NodeSystemTypeLabel: string(deckhousev1.SystemTypeDeckhouseEngine)},
		}})

		before := resourceVersions(ctx)
		Expect(MigrateSystemTypes(ctx, k8sClient)).To(Succeed())
		after := resourceVersions(ctx)

		for _, group := range groups {
			stored := &deckhousev1.NodeGroup{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: group.name}, stored)).To(Succeed())
			Expect(stored.Spec.SystemType).To(Equal(group.want), group.name)
		}
		for _, untouched := range []string{"NodeGroup/systemtype-unset", "NodeGroup/systemtype-deckhouseengine"} {
			Expect(after[untouched]).To(Equal(before[untouched]), untouched)
		}

		node := &corev1.Node{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "systemtype-node-immutable"}, node)).To(Succeed())
		Expect(node.Labels).To(HaveKeyWithValue(common.NodeSystemTypeLabel, string(deckhousev1.SystemTypeDeckhouseEngine)))
		Expect(node.Labels).To(HaveKeyWithValue(common.NodeGroupLabel, "worker"), "the patch must keep the other labels")
		for _, untouched := range []string{"Node/systemtype-node-unlabelled", "Node/systemtype-node-deckhouseengine", "Node/systemtype-node-mutable"} {
			Expect(after[untouched]).To(Equal(before[untouched]), untouched)
		}

		Expect(MigrateSystemTypes(ctx, k8sClient)).To(Succeed())
		Expect(resourceVersions(ctx)).To(Equal(after), "the second run must write nothing")
	})
})

func createObject(ctx context.Context, obj client.Object) {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	DeferCleanup(func(ctx SpecContext) {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
	})
}

func resourceVersions(ctx context.Context) map[string]string {
	GinkgoHelper()

	versions := map[string]string{}
	groups := &deckhousev1.NodeGroupList{}
	Expect(k8sClient.List(ctx, groups)).To(Succeed())
	for _, group := range groups.Items {
		versions["NodeGroup/"+group.Name] = group.ResourceVersion
	}
	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(ctx, nodes)).To(Succeed())
	for _, node := range nodes.Items {
		versions["Node/"+node.Name] = node.ResourceVersion
	}
	return versions
}
