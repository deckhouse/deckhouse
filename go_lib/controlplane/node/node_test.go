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
package node

import (
	"context"
	"errors"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNodeManager(t *testing.T) {

	t.Run("Existing Labels With Empty Taints", ExistingLabelsWithEmptyTaints)
	t.Run("NoExisting Taints", NoExistingTaints)
	t.Run("Existing Taint", ExistingTaint)
	t.Run("Node Does Not Exist", NodeDoesNotExist)
	t.Run("Label Update Fail", LabelUpdateFail)
	t.Run("Taints Update Fail", TaintsUpdateFail)
	t.Run("Overwrite Existing Taint", OverwriteExistingTaint)
	t.Run("Preserves Existing Labels", PreservesExistingLabels)
	t.Run("Test NewNodeManager", TestNewNodeManager)
}

func ExistingLabelsWithEmptyTaints(t *testing.T) {

	nodeName := "test-node"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{"existing-label": "value"},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	if val, exists := updatedNode.Labels[constants.ControlPlaneLabelKey]; !exists || val != "" {
		t.Errorf("Expected label %s='' to be added, got labels: %v", constants.ControlPlaneLabelKey, updatedNode.Labels)
	}

	if val, exists := updatedNode.Labels["existing-label"]; !exists || val != "value" {
		t.Errorf("Expected existing label to be preserved, got: %v", val)
	}

	foundTaint := false
	for _, taint := range updatedNode.Spec.Taints {
		if taint.Key == constants.ControlPlaneTaintKey &&
			taint.Effect == corev1.TaintEffectNoSchedule &&
			taint.Value == "" {
			foundTaint = true
			break
		}
	}
	if !foundTaint {
		t.Errorf("Expected taint '%s' '%s' '<empty_value>' to be added, got taints: %v", constants.ControlPlaneTaintKey, corev1.TaintEffectNoSchedule, updatedNode.Spec.Taints)
	}
}

func NoExistingTaints(t *testing.T) {

	nodeName := "clean-node"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	if len(updatedNode.Spec.Taints) != 1 {
		t.Fatalf("Expected exactly 1 taint, got %d taints: %v",
			len(updatedNode.Spec.Taints), updatedNode.Spec.Taints)
	}

	taint := updatedNode.Spec.Taints[0]
	if taint.Key != constants.ControlPlaneTaintKey {
		t.Errorf("Expected taint key %s, got %s",
			constants.ControlPlaneTaintKey, taint.Key)
	}
	if taint.Effect != corev1.TaintEffectNoSchedule {
		t.Errorf("Expected taint effect '%s', got '%s'",
			corev1.TaintEffectNoSchedule, taint.Effect)
	}
	if taint.Value != "" {
		t.Errorf("Expected empty taint value, got '%s'", taint.Value)
	}
}

func ExistingTaint(t *testing.T) {

	nodeName := "node-with-taints"
	existingTaintKey := "existing-taint"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    existingTaintKey,
					Value:  "value",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	if len(updatedNode.Spec.Taints) != 2 {
		t.Fatalf("Expected 2 taints, got %d taints: %v",
			len(updatedNode.Spec.Taints), updatedNode.Spec.Taints)
	}

	foundControlPlaneTaint := false
	foundExistingTaint := false

	for _, taint := range updatedNode.Spec.Taints {
		if taint.Key == constants.ControlPlaneTaintKey && taint.Effect == corev1.TaintEffectNoSchedule && taint.Value == "" {
			foundControlPlaneTaint = true
		}
		if taint.Key == existingTaintKey {
			foundExistingTaint = true
		}
	}

	if !foundControlPlaneTaint {
		t.Errorf("Control plane taint not found or differs from expected '%s' '%s' '<empty_value>'", constants.ControlPlaneTaintKey, corev1.TaintEffectNoSchedule)
	}
	if !foundExistingTaint {
		t.Errorf("Existing taint with key '%s' not preserved", existingTaintKey)
	}
}

func NodeDoesNotExist(t *testing.T) {

	client := fake.NewClientset()
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane("non-existent-node")

	if err == nil {
		t.Fatal("Expected error when node doesn't exist")
	}
}

func LabelUpdateFail(t *testing.T) {

	nodeName := "test-node"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{},
		},
	}

	client := fake.NewClientset(initialNode)

	// Track patch calls to fail only label patches
	patchCount := 0
	client.PrependReactor("patch", "nodes",
		func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			patchCount++
			// Fail only the first patch (label patch)
			if patchCount == 1 {
				return true, nil, errors.New("simulated label patch error")
			}
			// Allow subsequent patches to succeed
			return false, nil, nil
		})

	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)
	if err == nil {
		t.Fatal("Expected error when label patch fails")
	}

	// Verify no taint patch was attempted
	if patchCount > 1 {
		t.Error("Taint patch should not have been attempted after label patch failed")
	}
}

func TaintsUpdateFail(t *testing.T) {
	// Given: Node with existing labels
	nodeName := "test-node"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{"existing": "label"},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{},
		},
	}

	client := fake.NewClientset(initialNode)

	// Track patch calls
	patchCount := 0
	client.PrependReactor("patch", "nodes",
		func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			patchCount++
			// First patch (label update) succeeds
			// Second patch (taint update) fails
			if patchCount == 2 {
				return true, nil, errors.New("simulated taint patch error")
			}
			// Allow first patch to succeed
			return false, nil, nil
		})

	nodeManager := NewNodeManager(client)

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err == nil {
		t.Fatal("Expected error when taint patch fails")
	}

	// Verify exactly 2 patches were attempted
	if patchCount != 2 {
		t.Errorf("Expected 2 patch attempts, got %d", patchCount)
	}
}

func OverwriteExistingTaint(t *testing.T) {

	nodeName := "node-with-existing-cp-taint"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    constants.ControlPlaneTaintKey,
					Value:  "old-value",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	controlPlaneTaintCount := 0
	for _, taint := range updatedNode.Spec.Taints {
		if taint.Key == constants.ControlPlaneTaintKey {
			controlPlaneTaintCount++
			if taint.Value != "" {
				t.Errorf("Expected empty taint value, got '%s'", taint.Value)
			}
			if taint.Effect != corev1.TaintEffectNoSchedule {
				t.Errorf("Expected taint effect '%s', got '%s'",
					corev1.TaintEffectNoSchedule, taint.Effect)
			}
		}
	}

	if controlPlaneTaintCount != 1 {
		t.Errorf("Expected exactly 1 control plane taint, got %d", controlPlaneTaintCount)
	}
}

func PreservesExistingLabels(t *testing.T) {

	nodeName := "node-with-labels"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"label1":                 "value1",
				"label2":                 "value2",
				"kubernetes.io/hostname": "node-1",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	err := nodeManager.MarkAsControlPlane(nodeName)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	expectedLabels := map[string]string{
		"label1":                       "value1",
		"label2":                       "value2",
		"kubernetes.io/hostname":       "node-1",
		constants.ControlPlaneLabelKey: "",
	}

	for key, expectedValue := range expectedLabels {
		actualValue, exists := updatedNode.Labels[key]
		if !exists {
			t.Errorf("Expected label %s not found", key)
		} else if actualValue != expectedValue {
			t.Errorf("For label %s, expected value '%s', got '%s'",
				key, expectedValue, actualValue)
		}
	}

	if len(updatedNode.Labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d labels: %v",
			len(expectedLabels), len(updatedNode.Labels), updatedNode.Labels)
	}
}

type mockFailingClient struct {
	*fake.Clientset
	failUpdate       bool
	failSecondUpdate bool
	updateCount      int
}

func (m *mockFailingClient) CoreV1() corev1client.CoreV1Interface {
	return &mockCoreV1{
		CoreV1Interface: m.Clientset.CoreV1(),
		mockClient:      m,
	}
}

type mockCoreV1 struct {
	corev1client.CoreV1Interface
	mockClient *mockFailingClient
}

func (m *mockCoreV1) Nodes() corev1client.NodeInterface {
	return &mockNodeInterface{
		NodeInterface: m.CoreV1Interface.Nodes(),
		mockClient:    m.mockClient,
	}
}

type mockNodeInterface struct {
	corev1client.NodeInterface
	mockClient *mockFailingClient
}

func (m *mockNodeInterface) Update(ctx context.Context, node *corev1.Node, opts metav1.UpdateOptions) (*corev1.Node, error) {
	m.mockClient.updateCount++

	if m.mockClient.failUpdate ||
		(m.mockClient.failSecondUpdate && m.mockClient.updateCount == 2) {
		return nil, errors.New("mock update error")
	}

	return m.NodeInterface.Update(ctx, node, opts)
}

func TestNewNodeManager(t *testing.T) {
	client := fake.NewClientset()
	nodeManager := NewNodeManager(client)

	if nodeManager == nil {
		t.Error("Expected NodeManager to be created")
	}
}
