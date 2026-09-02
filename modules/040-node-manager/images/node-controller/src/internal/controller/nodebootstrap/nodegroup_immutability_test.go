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

package nodebootstrap

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const systemTypeFieldOwner = client.FieldOwner("systemtype-immutability-test")

// User story: the CRD schema refuses changing a NodeGroup's systemType once
// named (setting it on an unset group stays allowed — bootstrap relies on it);
// envtest runs no webhooks, so only the CRD rule is exercised here.
var _ = Describe("NodeGroup systemType immutability", func() {
	It("accepts a group created with a systemType, and one created without", func(ctx context.Context) {
		immutable := testenv.CreateImmutableNodeGroup(ctx, k8sClient, testenv.UniqueName("born-imm"))
		Expect(immutable.Spec.SystemType).To(Equal(deckhousev1.SystemTypeImmutable))

		bashible := createBashibleNodeGroup(ctx, testenv.UniqueName("born-mut"))
		Expect(bashible.Spec.SystemType).To(BeEmpty())
	})

	// Ginkgo runs one apiserver rule per row: what a group may still say about
	// its systemType once it has named one, and the transition the immutable
	// bootstrap depends on.
	DescribeTable("changing the systemType of a group",
		func(ctx context.Context, named, to deckhousev1.SystemType, allowed bool) {
			ng := testenv.CreateImmutableNodeGroup(ctx, k8sClient, testenv.UniqueName("st"),
				func(ng *deckhousev1.NodeGroup) { ng.Spec.SystemType = named })

			ng.Spec.SystemType = to
			err := k8sClient.Update(ctx, ng)
			if !allowed {
				Expect(err).To(MatchError(ContainSubstring("systemType")))
				return
			}
			Expect(err).To(Succeed())

			fresh := &deckhousev1.NodeGroup{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ng.Name}, fresh)).To(Succeed())
			Expect(fresh.Spec.SystemType).To(Equal(to))
		},
		Entry("a group that named none records Immutable", deckhousev1.SystemType(""), deckhousev1.SystemTypeImmutable, true),
		Entry("a group that named none records Mutable", deckhousev1.SystemType(""), deckhousev1.SystemTypeMutable, true),
		Entry("a named systemType cannot be cleared", deckhousev1.SystemTypeImmutable, deckhousev1.SystemType(""), false),
		Entry("a named systemType cannot be changed to another", deckhousev1.SystemTypeImmutable, deckhousev1.SystemTypeMutable, false),
	)

	// A merge patch removes a field by setting it to null, which is not an
	// update the typed client can express.
	It("refuses a merge patch that nulls the systemType", func(ctx context.Context) {
		ng := testenv.CreateImmutableNodeGroup(ctx, k8sClient, testenv.UniqueName("nonull"))

		patch := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"systemType":null}}`))
		Expect(k8sClient.Patch(ctx, ng, patch)).To(MatchError(ContainSubstring("systemType")))
	})

	// An apply that stops sending a field it owns removes it, which is how a
	// GitOps pipeline drops one without ever naming it.
	It("refuses an apply that stops sending the systemType", func(ctx context.Context) {
		name := testenv.UniqueName("noapply")

		Expect(k8sClient.Patch(ctx, applyNodeGroup(name, deckhousev1.SystemTypeImmutable),
			client.Apply, systemTypeFieldOwner, client.ForceOwnership)).To(Succeed())
		DeferCleanup(func(ctx context.Context) {
			_ = k8sClient.Delete(ctx, &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: name}})
		})

		Expect(k8sClient.Patch(ctx, applyNodeGroup(name, ""),
			client.Apply, systemTypeFieldOwner, client.ForceOwnership)).To(MatchError(ContainSubstring("systemType")))
	})

	It("allows an unrelated change to either group", func(ctx context.Context) {
		immutable := testenv.CreateImmutableNodeGroup(ctx, k8sClient, testenv.UniqueName("edit"))
		immutable.Spec.CloudInstances.MaxPerZone = 5
		Expect(k8sClient.Update(ctx, immutable)).To(Succeed())

		bashible := createBashibleNodeGroup(ctx, testenv.UniqueName("edit"))
		bashible.Spec.CloudInstances.MaxPerZone = 5
		Expect(k8sClient.Update(ctx, bashible)).To(Succeed())
	})
})

func createBashibleNodeGroup(ctx context.Context, name string) *deckhousev1.NodeGroup {
	GinkgoHelper()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone:     1,
				MaxPerZone:     3,
				ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, ng)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })

	fresh := &deckhousev1.NodeGroup{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(Succeed())
	return fresh
}

// applyNodeGroup builds the partial object a server-side apply sends: only the
// fields the applier claims, so leaving systemType out is what drops it.
func applyNodeGroup(name string, systemType deckhousev1.SystemType) *unstructured.Unstructured {
	spec := map[string]any{
		"nodeType": string(deckhousev1.NodeTypeCloudEphemeral),
		"cloudInstances": map[string]any{
			"minPerZone":     int64(1),
			"maxPerZone":     int64(3),
			"classReference": map[string]any{"kind": "DVPInstanceClass", "name": "worker"},
		},
	}
	if systemType != "" {
		spec["systemType"] = string(systemType)
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": deckhousev1.GroupVersion.String(),
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}
