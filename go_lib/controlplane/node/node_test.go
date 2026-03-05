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
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/kubernetes/fake"
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

// TestNodeManager_MarkAsControlPlane_Success tests successful marking of node as control plane
func ExistingLabelsWithEmptyTaints(t *testing.T) {
	// Given
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

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify node was updated
	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	// Verify control plane label was added
	if val, exists := updatedNode.Labels[constants.ControlPlaneLabelKey]; !exists || val != "" {
		t.Errorf("Expected label %s='' to be added, got labels: %v", constants.ControlPlaneLabelKey, updatedNode.Labels)
	}

	// Verify existing labels are preserved
	if val, exists := updatedNode.Labels["existing-label"]; !exists || val != "value" {
		t.Errorf("Expected existing label to be preserved, got: %v", val)
	}

	// Verify control plane taint was added
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

// TestNodeManager_MarkAsControlPlane_NodeWithNoTaintsWasTaintedSuccessfullyWithNewTaint
// This is the specific test case requested
func NoExistingTaints(t *testing.T) {
	// Given: Node with no existing taints
	nodeName := "clean-node"
	initialNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{}, // Empty taints slice
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the node now has exactly one taint
	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	if len(updatedNode.Spec.Taints) != 1 {
		t.Fatalf("Expected exactly 1 taint, got %d taints: %v",
			len(updatedNode.Spec.Taints), updatedNode.Spec.Taints)
	}

	// Verify it's the correct control plane taint
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

// TestNodeManager_MarkAsControlPlane_NodeWithExistingTaints tests merging with existing taints
func ExistingTaint(t *testing.T) {
	// Given: Node with existing taints
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

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify both taints exist
	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}

	if len(updatedNode.Spec.Taints) != 2 {
		t.Fatalf("Expected 2 taints, got %d taints: %v",
			len(updatedNode.Spec.Taints), updatedNode.Spec.Taints)
	}

	// Verify both taints are present
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

// TestNodeManager_MarkAsControlPlane_NodeNotFound tests error when node doesn't exist
func NodeDoesNotExist(t *testing.T) {
	// Given: No node in the fake client
	client := fake.NewClientset()
	nodeManager := NewNodeManager(client)

	// When
	err := nodeManager.MarkAsControlPlane("non-existent-node")

	// Then
	if err == nil {
		t.Fatal("Expected error when node doesn't exist")
	}
}

// TestNodeManager_MarkAsControlPlane_LabelUpdateFails tests label update failure
func LabelUpdateFail(t *testing.T) {
	// Given: Mock client that fails on update
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

	// Create a failing mock client
	mockClient := &mockFailingClient{
		Clientset:  fake.NewClientset(initialNode),
		failUpdate: true,
	}

	nodeManager := NewNodeManager(mockClient)

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err == nil {
		t.Fatal("Expected error when label update fails")
	}
}

// TestNodeManager_MarkAsControlPlane_TaintUpdateFails tests taint update failure
func TaintsUpdateFail(t *testing.T) {
	// Given: Mock client that fails on second update (taint update)
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

	// Create a mock that fails on the second Update call
	mockClient := &mockFailingClient{
		Clientset:        fake.NewClientset(initialNode),
		failSecondUpdate: true,
		updateCount:      0,
	}

	nodeManager := NewNodeManager(mockClient)

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err == nil {
		t.Fatal("Expected error when taint update fails")
	}
}

// TestNodeManager_MarkAsControlPlane_OverwritesExistingControlPlaneTaint tests taint replacement
func OverwriteExistingTaint(t *testing.T) {
	// Given: Node with existing control plane taint with different effect
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
					Effect: corev1.TaintEffectNoExecute, // Different effect
				},
			},
		},
	}

	client := fake.NewClientset(initialNode)
	nodeManager := NewNodeManager(client)

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify only one control plane taint exists with correct values
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

// TestNodeManager_MarkAsControlPlane_PreservesExistingLabels tests label preservation
func PreservesExistingLabels(t *testing.T) {
	// Given: Node with multiple existing labels
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

	// When
	err := nodeManager.MarkAsControlPlane(nodeName)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify all existing labels are preserved plus control plane label
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

// Mock client for testing failures
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

// Test helpers
func TestNewNodeManager(t *testing.T) {
	client := fake.NewClientset()
	nodeManager := NewNodeManager(client)

	if nodeManager == nil {
		t.Error("Expected NodeManager to be created")
	}
}
