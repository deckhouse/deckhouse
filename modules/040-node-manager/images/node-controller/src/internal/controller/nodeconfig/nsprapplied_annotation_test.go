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

package nodeconfig

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// immutableGroupName is the one group these tests call Engine; everything else
// is a bashible node reporting through its annotation.
const immutableGroupName = "workers-imm"

// annotationScheme is all this source reads: Nodes, and nothing else. When the
// last bashible node is gone, this file and its subject go together and take
// corev1 out of this package's test dependencies with them.
func annotationScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

// bashibleNode builds a Node of a mutable group with the annotation its bashible
// step leaves behind. An empty written list means no annotation at all: the step
// removes the key rather than writing an empty value.
//
// The key is spelled out rather than taken from staticPodsAnnotation on purpose:
// it is a contract with a component in another image, so a test that renamed
// itself along with the constant would prove nothing.
func bashibleNode(name, group, written string) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   name,
		Labels: map[string]string{nodecommon.NodeGroupLabel: group},
	}}
	if written != "" {
		node.Annotations = map[string]string{"node.deckhouse.io/static-pods": written}
	}
	return node
}

// A bashible node has no NodeConfig; its step writes the manifests and lists
// what it wrote in an annotation. That list is the whole of what this source
// knows, so everything in it is applied and nothing is ever a refusal.
func TestAnnotationOutcomesCountWhatTheStepWrote(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(annotationScheme(t)).WithObjects(
		bashibleNode("mutable-0", "workers", "registry-agent"),
		bashibleNode("mutable-1", "workers", "registry-agent,something-else"),
		// The step has not run, or wrote nothing: no annotation at all, counted
		// neither way.
		bashibleNode("mutable-2", "workers", ""),
	).Build()

	outcomes, err := readAnnotationOutcomes(context.Background(), cl, []string{immutableGroupName})
	require.NoError(t, err)

	require.Equal(t, int32(2), outcomes["registry-agent"].applied)
	require.Equal(t, int32(1), outcomes["something-else"].applied)
	require.Equal(t, int32(0), outcomes["registry-agent"].failed,
		"a failed step shows up in the node's configuration checksum, never here")
}

// A node moved from bashible to Engine keeps the annotation its old step left.
// It reports through its NodeConfig now, so counting the leftover as well would
// call one node two applied nodes.
func TestAnnotationOutcomesIgnoreImmutableNodes(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(annotationScheme(t)).WithObjects(
		bashibleNode("imm-0", immutableGroupName, "registry-agent"),
		bashibleNode("mutable-0", "workers", "registry-agent"),
	).Build()

	outcomes, err := readAnnotationOutcomes(context.Background(), cl, []string{immutableGroupName})
	require.NoError(t, err)
	require.Equal(t, int32(1), outcomes["registry-agent"].applied)
}

// Spaces around a name are not a different name. The step writes none, but this
// annotation is the sort of thing an operator edits by hand at 3am, and a stray
// space would silently drop that node out of appliedNodes.
func TestAnnotationOutcomesTolerateSpaces(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(annotationScheme(t)).WithObjects(
		bashibleNode("mutable-0", "workers", "registry-agent, something-else"),
	).Build()

	outcomes, err := readAnnotationOutcomes(context.Background(), cl, []string{immutableGroupName})
	require.NoError(t, err)
	require.Equal(t, int32(1), outcomes["registry-agent"].applied)
	require.Equal(t, int32(1), outcomes["something-else"].applied)
}

// An annotation edited by hand can carry an empty element, and Split hands one
// back as "". Counted, it becomes an outcome keyed on no object at all, which no
// NodeStaticPodRequest ever collects and nothing ever clears.
func TestAnnotationOutcomesIgnoreEmptyNames(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(annotationScheme(t)).WithObjects(
		bashibleNode("mutable-0", "workers", "registry-agent,,something-else"),
		bashibleNode("mutable-1", "workers", ","),
	).Build()

	outcomes, err := readAnnotationOutcomes(context.Background(), cl, []string{immutableGroupName})
	require.NoError(t, err)

	require.NotContains(t, outcomes, "")
	require.Equal(t, int32(1), outcomes["registry-agent"].applied)
	require.Equal(t, int32(1), outcomes["something-else"].applied)
}

// A bashible node reports through its annotation and has no NodeConfig at all,
// so the merge of the second source is what keeps it counted; without it such a
// node reads as one that never wrote the pod.
func TestNSPRStatusCountsTheBashibleNodes(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup(immutableGroupName),
			bashibleNode("mutable-0", "workers", "registry-agent"),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, int32(1), fresh.Status.AppliedNodes)
	require.Equal(t, phaseReady, fresh.Status.Phase)
}

// Static pods reach bashible groups too — their step writes them — so a group
// missing from matchedNodeGroups while its nodes are counted in appliedNodes is
// a status that contradicts itself.
func TestNSPRStatusMatchesABashibleOnlyGroup(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{
		NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"workers"}},
	})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup(immutableGroupName),
			&v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "workers"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, SystemType: v1.SystemTypeMutable},
			},
			bashibleNode("mutable-0", "workers", "registry-agent"),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, []string{"workers"}, fresh.Status.MatchedNodeGroups)
	require.Equal(t, int32(1), fresh.Status.AppliedNodes)
}
