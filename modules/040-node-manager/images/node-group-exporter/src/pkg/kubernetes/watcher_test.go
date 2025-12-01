/*
Copyright 2025 Flant JSC

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

package kubernetes

import (
	"testing"

	ngv1 "node-group-exporter/internal/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// MockEventHandler is a mock implementation of EventHandler
type MockEventHandler struct {
	mock.Mock
}

func (m *MockEventHandler) OnNodeGroupAddOrUpdate(nodegroup *ngv1.NodeGroup) { m.Called(nodegroup) }
func (m *MockEventHandler) OnNodeGroupDelete(nodegroup *ngv1.NodeGroup)      { m.Called(nodegroup) }
func (m *MockEventHandler) OnNodeAddOrUpdate(node *v1.Node)                  { m.Called(node) }
func (m *MockEventHandler) OnNodeDelete(node *v1.Node)                       { m.Called(node) }

// test helpers
func newWatcherForTest(t *testing.T) (*Watcher, *MockEventHandler) {
	t.Helper()
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	w, err := NewWatcher(clientset, restConfig, mockHandler, log.Default())
	assert.NoError(t, err)
	return w, mockHandler
}

// TestWatcher tests the Watcher
func TestWatcher(t *testing.T) {
	w, _ := newWatcherForTest(t)
	assert.NotNil(t, w.clientset)
	assert.NotNil(t, w.dynamicClient)
	assert.NotNil(t, w.eventHandler)
	assert.NotNil(t, w.stopCh)
}

// TestConvertToNodeGroup tests the ConvertToNodeGroup function
func TestConvertToNodeGroup(t *testing.T) {

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]any{
				"name":      "test-nodegroup",
				"namespace": "default",
				"labels":    map[string]any{"env": "test"},
			},
			"spec": map[string]any{
				"nodeType": "Cloud",
				"cloudInstances": map[string]any{
					"maxPerZone": int64(5),
					"minPerZone": int64(1),
					"zones":      []any{"zone-a", "zone-b"},
				},
			},
			"status": map[string]any{"desired": int64(4), "ready": int64(3)},
		},
	}

	result, err := ConvertToNodeGroup(obj)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-nodegroup", result.Name)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, "Cloud", result.Spec.NodeType.String())
	assert.NotNil(t, result.Spec.CloudInstances)
	assert.NotNil(t, result.Spec.CloudInstances.MaxPerZone)
	assert.Equal(t, int32(5), *result.Spec.CloudInstances.MaxPerZone)
	assert.NotNil(t, result.Spec.CloudInstances.MinPerZone)
	assert.Equal(t, int32(1), *result.Spec.CloudInstances.MinPerZone)
	assert.Equal(t, []string{"zone-a", "zone-b"}, result.Spec.CloudInstances.Zones)
	assert.Equal(t, int32(4), result.Status.Desired)
	assert.Equal(t, int32(3), result.Status.Ready)

	invalidObj := "not a nodegroup"
	result, err = ConvertToNodeGroup(invalidObj)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

// TestWatcherEventHandler tests the EventHandler integration
func TestWatcherEventHandler(t *testing.T) {
	w, mh := newWatcherForTest(t)

	mh.On("OnNodeGroupAddOrUpdate", mock.Anything)
	mh.On("OnNodeAddOrUpdate", mock.Anything)

	ng := &ngv1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "test-group", Namespace: "default"}}
	n := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test-node"}}

	if w.eventHandler != nil {
		w.eventHandler.OnNodeGroupAddOrUpdate(ng)
		w.eventHandler.OnNodeAddOrUpdate(n)
	}
	mh.AssertExpectations(t)
}
