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

package nodegroup

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	ConditionTypeReady                        = ngcommon.ConditionTypeReady
	ConditionTypeUpdating                     = ngcommon.ConditionTypeUpdating
	ConditionTypeWaitingForDisruptiveApproval = ngcommon.ConditionTypeWaitingForDisruptiveApproval
	ConditionTypeError                        = ngcommon.ConditionTypeError
	ConditionTypeScaling                      = ngcommon.ConditionTypeScaling
	ConditionTypeFrozen                       = ngcommon.ConditionTypeFrozen
	NodeGroupLabel                            = ngcommon.NodeGroupLabel
	ConfigurationChecksumAnnotation           = ngcommon.ConfigurationChecksumAnnotation
	MachineNamespace                          = ngcommon.MachineNamespace
	ConfigurationChecksumsSecretName          = ngcommon.ConfigurationChecksumsSecretName
	CloudProviderSecretName                   = ngcommon.CloudProviderSecretName
	DisruptionRequiredAnnotation              = ngcommon.DisruptionRequiredAnnotation
	ApprovedAnnotation                        = ngcommon.ApprovedAnnotation
)

func ngTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	return scheme
}

func ngTestReconciler(objs ...runtime.Object) *NodeGroupStatusReconciler {
	scheme := ngTestScheme()
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()
	return &NodeGroupStatusReconciler{
		Client:   cl,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(100),
	}
}

func ngTestReconcilerWithUnstructured(typed []runtime.Object, unstruct []*unstructured.Unstructured) *NodeGroupStatusReconciler {
	scheme := ngTestScheme()
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(typed...).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()
	for _, u := range unstruct {
		_ = cl.Create(context.Background(), u)
	}
	return &NodeGroupStatusReconciler{
		Client:   cl,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(100),
	}
}

func reconcileNG(t *testing.T, r *NodeGroupStatusReconciler, name string) *v1.NodeGroup {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, ng); err != nil {
		t.Fatalf("failed to get nodegroup: %v", err)
	}
	return ng
}

func findCond(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func assertCond(t *testing.T, conditions []metav1.Condition, condType string, status metav1.ConditionStatus) {
	t.Helper()
	c := findCond(conditions, condType)
	if c == nil {
		t.Fatalf("condition %s not found", condType)
	}
	if c.Status != status {
		t.Errorf("condition %s: expected status=%s, got %s", condType, status, c.Status)
	}
}

func assertCondMsg(t *testing.T, conditions []metav1.Condition, condType string, status metav1.ConditionStatus, msgSubstr string) {
	t.Helper()
	c := findCond(conditions, condType)
	if c == nil {
		t.Fatalf("condition %s not found", condType)
	}
	if c.Status != status {
		t.Errorf("condition %s: expected status=%s, got %s", condType, status, c.Status)
	}
	if msgSubstr != "" && c.Message != msgSubstr {
		t.Errorf("condition %s: expected message containing %q, got %q", condType, msgSubstr, c.Message)
	}
}

func assertNoCond(t *testing.T, conditions []metav1.Condition, condType string) {
	t.Helper()
	if findCond(conditions, condType) != nil {
		t.Errorf("condition %s should not exist", condType)
	}
}

func makeNode(name, ngName string, ready bool, checksum string) *corev1.Node {
	annotations := map[string]string{}
	if checksum != "" {
		annotations[ConfigurationChecksumAnnotation] = checksum
	}
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{NodeGroupLabel: ngName},
			Annotations: annotations,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: readyStatus},
			},
		},
	}
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func makeZonesSecret(zones string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: CloudProviderSecretName, Namespace: "kube-system"},
		Data:       map[string][]byte{"zones": []byte(zones)},
	}
}

func makeChecksumSecret(data map[string]string) *corev1.Secret {
	d := make(map[string][]byte)
	for k, v := range data {
		d[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigurationChecksumsSecretName, Namespace: MachineNamespace},
		Data:       d,
	}
}

func makeMCMMachineDeployment(name, ngName string, replicas int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "machine.sapcloud.io/v1alpha1",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": MachineNamespace,
				"labels":    map[string]interface{}{"node-group": ngName},
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
		},
	}
}

func makeCAPIMachineDeployment(name, ngName string, replicas int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta1",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": MachineNamespace,
				"labels":    map[string]interface{}{"node-group": ngName},
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
		},
	}
}

func makeMCMMachine(name, ngName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "machine.sapcloud.io/v1alpha1",
			"kind":       "Machine",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": MachineNamespace,
			},
			"spec": map[string]interface{}{
				"nodeTemplate": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							NodeGroupLabel: ngName,
						},
					},
				},
			},
		},
	}
}

func makeCAPIMachine(name, ngName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta1",
			"kind":       "Machine",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": MachineNamespace,
				"labels":    map[string]interface{}{"node-group": ngName},
			},
		},
	}
}

func makeFailedMCMMD(name, ngName string, replicas int64, failures []map[string]interface{}) *unstructured.Unstructured {
	md := makeMCMMachineDeployment(name, ngName, replicas)
	failedMachines := make([]interface{}, 0, len(failures))
	for _, f := range failures {
		failedMachines = append(failedMachines, f)
	}
	md.Object["status"] = map[string]interface{}{
		"failedMachines": failedMachines,
	}
	return md
}

func makeFrozenMCMMD(name, ngName string, replicas int64) *unstructured.Unstructured {
	md := makeMCMMachineDeployment(name, ngName, replicas)
	md.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Frozen",
				"status": "True",
			},
		},
	}
	return md
}

func makeNodeWithAnnotations(name, ngName string, ready bool, checksum string, extraAnnotations map[string]string) *corev1.Node {
	node := makeNode(name, ngName, ready, checksum)
	for k, v := range extraAnnotations {
		node.Annotations[k] = v
	}
	return node
}
