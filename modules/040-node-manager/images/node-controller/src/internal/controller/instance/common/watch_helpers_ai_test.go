//go:build ai_tests

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func staticNode(labels map[string]string, annotations map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func TestAI_StaticNodeUpdatePredicate_AnnotationAdded_Fires(t *testing.T) {
	t.Parallel()
	pred := StaticNodeEventPredicate()
	oldNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, nil)
	newNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, map[string]string{"cluster.x-k8s.io/machine": "m1"})
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}))
}

func TestAI_StaticNodeUpdatePredicate_AnnotationRemoved_Fires(t *testing.T) {
	t.Parallel()
	pred := StaticNodeEventPredicate()
	oldNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, map[string]string{"cluster.x-k8s.io/machine": "m1"})
	newNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, nil)
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}))
}

func TestAI_StaticNodeUpdatePredicate_LabelChangeOnStaticNode_Fires(t *testing.T) {
	t.Parallel()
	pred := StaticNodeEventPredicate()
	oldNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, nil)
	newNode := staticNode(map[string]string{"node.deckhouse.io/type": "Static", "extra": "label"}, nil)
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}))
}

func TestAI_StaticNodeUpdatePredicate_NoChange_DoesNotFire(t *testing.T) {
	t.Parallel()
	pred := StaticNodeEventPredicate()
	node := staticNode(map[string]string{"node.deckhouse.io/type": "Static"}, nil)
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: node, ObjectNew: node}))
}

func TestAI_StaticNodeUpdatePredicate_NonStatic_DoesNotFire(t *testing.T) {
	t.Parallel()
	pred := StaticNodeEventPredicate()
	oldNode := staticNode(map[string]string{"node.deckhouse.io/type": "Worker"}, nil)
	newNode := staticNode(map[string]string{"node.deckhouse.io/type": "Worker", "extra": "x"}, nil)
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}))
}
