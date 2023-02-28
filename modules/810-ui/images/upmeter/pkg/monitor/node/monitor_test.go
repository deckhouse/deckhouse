/*
Copyright 2023 Flant JSC

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

package node

import (
	"context"
	"testing"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

func generateTestNode(t *testing.T, name string) *unstructured.Unstructured {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	convertedNode, err := runtime.DefaultUnstructuredConverter.ToUnstructured(node)
	if err != nil {
		t.Fatalf("cannot convert unstructured to corev1.Node: %v", err)
	}
	obj := &unstructured.Unstructured{}
	obj.Object = convertedNode
	return obj
}

func TestNodesMonitor(t *testing.T) {
	fakeClient := klient.NewFake(nil)

	_, err := fakeClient.Dynamic().
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "nodes"}).
		Create(context.TODO(), generateTestNode(t, "node-1"), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("cannot create a node: %v", err)
	}

	monitor := NewMonitor(fakeClient, logrus.NewEntry(logrus.New()))
	if err := monitor.Start(context.TODO()); err != nil {
		t.Fatalf("failed to start the nodes monitor: %v", err)
	}

	// #1 Fresh start
	nodes, err := monitor.List()
	if err != nil {
		t.Fatalf("failed to get nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("there should be a single node on the start, got %d", len(nodes))
	}

	// #2 Add node
	_, err = fakeClient.Dynamic().
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "nodes"}).
		Create(context.TODO(), generateTestNode(t, "node-2"), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("cannot create a node: %v", err)
	}

	// The inifinity will be capped by test timeout
	err = wait.PollInfinite(time.Millisecond, func() (bool, error) {
		nodes, err = monitor.List()
		if err != nil {
			return false, err
		}
		return len(nodes) == 2, nil
	})
	if err != nil {
		t.Fatalf(err.Error())
	}

	// #3 Delete node
	err = fakeClient.Dynamic().
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "nodes"}).
		Delete(context.TODO(), "node-2", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("cannot create a node: %v", err)
	}

	// The inifinity will be capped by test timeout
	err = wait.PollInfinite(time.Millisecond, func() (bool, error) {
		nodes, err = monitor.List()
		if err != nil {
			return false, err
		}
		return len(nodes) == 1, nil
	})
	if err != nil {
		t.Fatalf(err.Error())
	}
}
