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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

const testCloudEphemeralNodeTypeValue = "CloudEphemeral"

func TestStaticNodeEventPredicateCreateDelete(t *testing.T) {
	t.Parallel()

	pred, ok := StaticNodeEventPredicate().(predicate.Funcs)
	require.True(t, ok)

	require.True(t, pred.CreateFunc(event.CreateEvent{Object: staticNode("static-1", nil)}))
	require.True(t, pred.DeleteFunc(event.DeleteEvent{Object: staticNode("static-1", nil)}))

	require.False(t, pred.CreateFunc(event.CreateEvent{Object: makeNode("cloud-1", testCloudEphemeralNodeTypeValue, nil, false)}))
	require.False(t, pred.DeleteFunc(event.DeleteEvent{Object: makeNode("cloud-1", testCloudEphemeralNodeTypeValue, nil, false)}))

	require.False(t, pred.CreateFunc(event.CreateEvent{Object: nodeWithAnnotation("annotated-static")}))
	require.False(t, pred.DeleteFunc(event.DeleteEvent{Object: nodeWithAnnotation("annotated-static")}))
	require.False(t, pred.CreateFunc(event.CreateEvent{Object: nodeWithProviderID("caps-static", "static:///hash")}))
	require.False(t, pred.DeleteFunc(event.DeleteEvent{Object: nodeWithProviderID("caps-static", "static:///hash")}))
	require.False(t, pred.CreateFunc(event.CreateEvent{Object: nodeWithProviderIDAnnotation("caps-static", "static:///hash")}))
	require.False(t, pred.DeleteFunc(event.DeleteEvent{Object: nodeWithProviderIDAnnotation("caps-static", "static:///hash")}))
	require.True(t, pred.CreateFunc(event.CreateEvent{Object: nodeWithProviderID("plain-static", "static://")}))
	require.True(t, pred.DeleteFunc(event.DeleteEvent{Object: nodeWithProviderID("plain-static", "static://")}))
	require.True(t, pred.CreateFunc(event.CreateEvent{Object: nodeWithProviderID("empty-caps-id", "static:///")}))
	require.True(t, pred.DeleteFunc(event.DeleteEvent{Object: nodeWithProviderID("empty-caps-id", "static:///")}))
}

func TestStaticNodeEventPredicateUpdate(t *testing.T) {
	t.Parallel()

	pred, ok := StaticNodeEventPredicate().(predicate.Funcs)
	require.True(t, ok)

	tests := []struct {
		name   string
		oldObj *corev1.Node
		newObj *corev1.Node
		want   bool
	}{
		{
			name:   "annotation added",
			oldObj: staticNode("node", nil),
			newObj: nodeWithAnnotation("node"),
			want:   true,
		},
		{
			name:   "annotation removed",
			oldObj: nodeWithAnnotation("node"),
			newObj: staticNode("node", nil),
			want:   true,
		},
		{
			name:   "static label change",
			oldObj: staticNode("node-role", map[string]string{"role": "old"}),
			newObj: staticNode("node-role", map[string]string{"role": "new"}),
			want:   true,
		},
		{
			name:   "static label unchanged",
			oldObj: staticNode("node-same", map[string]string{"role": "same"}),
			newObj: staticNode("node-same", map[string]string{"role": "same"}),
			want:   false,
		},
		{
			name:   "non-static to non-static",
			oldObj: makeNode("node", testCloudEphemeralNodeTypeValue, nil, false),
			newObj: makeNode("node", testCloudEphemeralNodeTypeValue, nil, false),
			want:   false,
		},
		{
			name:   "static labeled node with CAPI annotation filtered",
			oldObj: nodeWithAnnotation("node"),
			newObj: nodeWithAnnotation("node"),
			want:   false,
		},
		{
			name:   "caps provider id added",
			oldObj: staticNode("node", nil),
			newObj: nodeWithProviderID("node", "static:///hash"),
			want:   true,
		},
		{
			name:   "caps provider id annotation added",
			oldObj: staticNode("node", nil),
			newObj: nodeWithProviderIDAnnotation("node", "static:///hash"),
			want:   true,
		},
		{
			name:   "plain static provider id remains static",
			oldObj: staticNode("node", nil),
			newObj: nodeWithProviderID("node", "static://"),
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := pred.UpdateFunc(event.UpdateEvent{ObjectOld: tt.oldObj, ObjectNew: tt.newObj})
			require.Equal(t, tt.want, got)
		})
	}
}

func staticNode(name string, extra map[string]string) *corev1.Node {
	return makeNode(name, string(deckhousev1.NodeTypeStatic), extra, false)
}

func nodeWithAnnotation(name string) *corev1.Node {
	return makeNode(name, string(deckhousev1.NodeTypeStatic), nil, true)
}

func nodeWithProviderID(name, providerID string) *corev1.Node {
	node := staticNode(name, nil)
	node.Spec.ProviderID = providerID

	return node
}

func nodeWithProviderIDAnnotation(name, providerID string) *corev1.Node {
	node := staticNode(name, nil)
	node.Annotations = map[string]string{
		nodecommon.ProviderIDAnnotation: providerID,
	}

	return node
}

func makeNode(name, nodeType string, extra map[string]string, annotated bool) *corev1.Node {
	labels := map[string]string{
		nodecommon.NodeTypeLabel: nodeType,
	}
	if extra != nil {
		for k, v := range extra {
			labels[k] = v
		}
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	if annotated {
		node.Annotations = map[string]string{
			nodecommon.CAPIMachineAnnotation: "machine",
		}
	}
	return node
}
