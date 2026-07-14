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

package kubeclient

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

func instanceGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Instance"}
}

func newInstance(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(instanceGVK())
	obj.SetName(name)
	return obj
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	return scheme
}

func TestGetNodesForNodeGroup(t *testing.T) {
	scheme := newScheme(t)
	matching := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "n1",
		Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
	}}
	other := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "n2",
		Labels: map[string]string{nodecommon.NodeGroupLabel: "master"},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(matching, other).Build()

	c := Client{Client: cl}
	nodes, err := c.GetNodesForNodeGroup(context.Background(), "worker")
	if err != nil {
		t.Fatalf("GetNodesForNodeGroup: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "n1" {
		t.Fatalf("expected only n1, got %+v", nodes)
	}
}

func TestGetConfigurationChecksums(t *testing.T) {
	scheme := newScheme(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodecommon.ConfigurationChecksumsSecretName,
			Namespace: nodecommon.MachineNamespace,
		},
		Data: map[string][]byte{"worker": []byte("abc"), "master": []byte("def")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(secret).Build()

	c := Client{Client: cl}
	checksums, err := c.GetConfigurationChecksums(context.Background())
	if err != nil {
		t.Fatalf("GetConfigurationChecksums: %v", err)
	}
	if checksums["worker"] != "abc" || checksums["master"] != "def" {
		t.Fatalf("unexpected checksums: %+v", checksums)
	}
}

func TestPatchNode_AppliesMergePatch(t *testing.T) {
	scheme := newScheme(t)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "n1",
		Annotations: map[string]string{"keep": "yes", "remove": "old"},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(node).Build()

	c := Client{Client: cl}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"add":    "new",
				"remove": nil,
			},
		},
	}
	if err := c.PatchNode(context.Background(), "n1", patch); err != nil {
		t.Fatalf("PatchNode: %v", err)
	}

	updated := &corev1.Node{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "n1"}, updated); err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updated.Annotations["add"] != "new" {
		t.Fatalf("expected add annotation to be set, got %+v", updated.Annotations)
	}
	if updated.Annotations["keep"] != "yes" {
		t.Fatalf("expected keep annotation to remain, got %+v", updated.Annotations)
	}
	if _, ok := updated.Annotations["remove"]; ok {
		t.Fatalf("expected remove annotation to be deleted, got %+v", updated.Annotations)
	}
}

func TestDeleteInstance_DeletesExisting(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(newInstance("n1")).
		Build()

	c := Client{Client: cl}
	if err := c.DeleteInstance(context.Background(), "n1"); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}

	remaining := &unstructured.Unstructured{}
	remaining.SetGroupVersionKind(instanceGVK())
	err := cl.Get(context.Background(), types.NamespacedName{Name: "n1"}, remaining)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected instance to be deleted, get returned: %v", err)
	}
}

func TestDeleteInstance_NotFoundIsIgnored(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	c := Client{Client: cl}
	if err := c.DeleteInstance(context.Background(), "missing"); err != nil {
		t.Fatalf("expected nil error for missing instance, got %v", err)
	}
}
