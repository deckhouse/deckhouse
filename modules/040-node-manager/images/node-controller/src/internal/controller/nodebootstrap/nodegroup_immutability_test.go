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

// User story: As a cluster operator, I want the API itself to refuse a change of
// a NodeGroup's systemType once the group has named one, so that nobody can turn
// an immutable group back into a bashible one — or the other way round — and have
// CAPI re-create every machine in it.
//
// A group that has named nothing yet is deliberately still writable: that is the
// write an immutable cluster performs on its own master NodeGroup, which the
// node-manager hook creates before dhctl applies the manifest naming its
// systemType. Denying it there turns a bootstrap into a ten-minute retry loop.
// The other half of the rule — do not adopt a group whose nodes bashible already
// configured — has to look at the group's nodes, so it lives in the
// node-controller webhook; envtest runs no webhooks, so what fails here is the
// CRD schema alone.
//
// This suite is the one that installs the NodeGroup CRD, which is why the CRD's
// own rule is exercised from here.
var _ = Describe("NodeGroup systemType immutability", func() {
	It("accepts a group created with a systemType, and one created without", func(ctx context.Context) {
		immutable := createImmutableNodeGroup(ctx, testenv.UniqueName("born-imm"))
		Expect(immutable.Spec.SystemType).To(Equal(deckhousev1.SystemTypeImmutable))

		bashible := createBashibleNodeGroup(ctx, testenv.UniqueName("born-mut"))
		Expect(bashible.Spec.SystemType).To(BeEmpty())
	})

	// The transition the immutable bootstrap depends on.
	It("lets a group that named no systemType record Immutable", func(ctx context.Context) {
		ng := createBashibleNodeGroup(ctx, testenv.UniqueName("adopt"))

		ng.Spec.SystemType = deckhousev1.SystemTypeImmutable
		Expect(k8sClient.Update(ctx, ng)).To(Succeed())

		fresh := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ng.Name}, fresh)).To(Succeed())
		Expect(fresh.Spec.SystemType).To(Equal(deckhousev1.SystemTypeImmutable))
	})

	It("lets a group that named no systemType record Mutable", func(ctx context.Context) {
		ng := createBashibleNodeGroup(ctx, testenv.UniqueName("adopt-mut"))

		ng.Spec.SystemType = deckhousev1.SystemTypeMutable
		Expect(k8sClient.Update(ctx, ng)).To(Succeed())
	})

	It("refuses to clear a systemType the group named", func(ctx context.Context) {
		ng := createImmutableNodeGroup(ctx, testenv.UniqueName("noclear"))

		ng.Spec.SystemType = ""
		Expect(k8sClient.Update(ctx, ng)).To(MatchError(ContainSubstring("systemType")))
	})

	It("refuses to change a systemType the group named", func(ctx context.Context) {
		ng := createImmutableNodeGroup(ctx, testenv.UniqueName("nochange"))

		ng.Spec.SystemType = deckhousev1.SystemTypeMutable
		Expect(k8sClient.Update(ctx, ng)).To(MatchError(ContainSubstring("systemType")))
	})

	// A merge patch removes a field by setting it to null, which is not an
	// update the typed client can express.
	It("refuses a merge patch that nulls the systemType", func(ctx context.Context) {
		ng := createImmutableNodeGroup(ctx, testenv.UniqueName("nonull"))

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
		immutable := createImmutableNodeGroup(ctx, testenv.UniqueName("edit"))
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
