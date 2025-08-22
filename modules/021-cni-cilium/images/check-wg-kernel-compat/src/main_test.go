/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testNodeName = "testcluster-worker-02334ee2-7694f-h9zgk"
)

func TestCheckAgentPodGeneration(t *testing.T) {
	testCases := []struct {
		name             string
		nodeName         string
		k8sObjects       []runtime.Object
		expectPodRestart bool
		expectSuccess    bool
	}{
		{
			name:             "DaemonSets_not_exist",
			nodeName:         testNodeName,
			k8sObjects:       []runtime.Object{},
			expectPodRestart: false,
			expectSuccess:    false,
		},
		{
			name:     "DaemonSets_exist_but_Pod_doesnt",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent",
						Namespace: ciliumNS,
						Labels: map[string]string{
							"label1": "value1",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									generationAnnotation: "1234567890",
								},
							},
						},
					},
				},
			},
			expectPodRestart: false,
			expectSuccess:    false,
		},
		{
			name:     "DS_and_2_Pod_exist",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent",
						Namespace: ciliumNS,
						Labels: map[string]string{
							"label1": "value1",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									generationAnnotation: "1234567890",
								},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-67890",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
			},
			expectPodRestart: false,
			expectSuccess:    false,
		},
		{
			name:     "DS_and_Pod_exist_but_DS_doesnt_have_gen",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent",
						Namespace: ciliumNS,
						Labels: map[string]string{
							"label1": "value1",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									generationAnnotation: "",
								},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
			},
			expectPodRestart: false,
			expectSuccess:    false,
		},
		{
			name:     "DS_and_Pod_exist_have_gens_and_its_equal",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent",
						Namespace: ciliumNS,
						Labels: map[string]string{
							"label1": "value1",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									generationAnnotation: "1234567890",
								},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
			},
			expectPodRestart: false,
			expectSuccess:    true,
		},
		{
			name:     "DS_and_Pod_exist_have_gens_and_its_doesnt_equal",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent",
						Namespace: ciliumNS,
						Labels: map[string]string{
							"label1": "value1",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									generationAnnotation: "1234567890",
								},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "0987654321",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
			},
			expectPodRestart: true,
			expectSuccess:    true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			_, _, isCurrentAgentPodGenerationDesired, err := checkAgentPodGeneration(fakeClientset, test.nodeName)
			podRestartNeeded := !isCurrentAgentPodGenerationDesired

			switch test.expectSuccess {
			case false:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case true:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				} else if test.expectPodRestart != podRestartNeeded {
					t.Fatalf("expected Pod need Restart but received another value")
				}
			}
		})
	}
}

func TestDeletePod(t *testing.T) {
	testCases := []struct {
		name          string
		podName       string
		k8sObjects    []runtime.Object
		expectSuccess bool
	}{
		{
			name:    "Pod_exist",
			podName: "agent-12345",
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:    "PodName_doesnt_exist",
			podName: "",
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
					},
				},
			},
			expectSuccess: false,
		},
		{
			name:          "Pod_doesnt_exist",
			podName:       "agent-12345",
			k8sObjects:    []runtime.Object{},
			expectSuccess: false,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			err := deletePod(fakeClientset, test.podName)
			// Check Pod in cluster
			pod, _ := fakeClientset.CoreV1().Pods(ciliumNS).Get(context.TODO(), test.podName, metav1.GetOptions{})

			switch test.expectSuccess {
			case false:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case true:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				} else if pod != nil {
					t.Fatal("expected pod will be deleted, but it still exist")
				}
			}
		})
	}
}

func TestWaitUntilNewPodCreatedAndBecomeReady(t *testing.T) {
	testCases := []struct {
		name           string
		nodeName       string
		scanIterations int
		k8sObjects     []runtime.Object
		expectSuccess  bool
	}{
		{
			name:           "Pod_does_not_exist",
			nodeName:       testNodeName,
			scanIterations: 1,
			k8sObjects:     []runtime.Object{},
			expectSuccess:  false,
		},
		{
			name:           "Pod_exist_but_in_terminating",
			nodeName:       testNodeName,
			scanIterations: 1,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name:           "New_pod_exist_but_not_ready",
			nodeName:       testNodeName,
			scanIterations: 1,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodInitialized,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name:           "Two_pods_exist",
			nodeName:       testNodeName,
			scanIterations: 1,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-67890",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name:           "New_pod_exist_and_ready",
			nodeName:       testNodeName,
			scanIterations: 1,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-12345",
						Namespace: ciliumNS,
						Annotations: map[string]string{
							generationAnnotation: "1234567890",
						},
						Labels: map[string]string{
							"app": "agent",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			expectSuccess: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			err := waitUntilNewPodCreatedAndBecomeReady(fakeClientset, test.nodeName, test.scanIterations)

			switch test.expectSuccess {
			case false:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case true:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// new
func TestIsMigrationSucceeded(t *testing.T) {
	testCases := []struct {
		name         string
		nodeName     string
		k8sObjects   []runtime.Object
		expectResult bool
	}{
		{
			name:         "Node_does_not_exist",
			nodeName:     testNodeName,
			k8sObjects:   []runtime.Object{},
			expectResult: false,
		},
		{
			name:     "Node_exists_but_migration_annotation_missing",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
				},
			},
			expectResult: false,
		},
		{
			name:     "Node_and_migration_annotation_exists",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
						Annotations: map[string]string{
							migrationSucceededAnnotation: "",
						},
					},
				},
			},
			expectResult: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			receivedResult := isMigrationSucceeded(fakeClientset, test.nodeName)

			switch test.expectResult {
			case false:
				if receivedResult {
					t.Fatalf("expected false but received true")
				} else {
					t.Logf("expected false and got it: %v", receivedResult)
				}
			case true:
				if !receivedResult {
					t.Fatalf("unexpected success: %v", receivedResult)
				}
			}
		})
	}
}

func TestAreSTSPodsPresentOnNode(t *testing.T) {
	testCases := []struct {
		name         string
		nodeName     string
		k8sObjects   []runtime.Object
		expectResult bool
	}{
		{
			name:         "No_one_pods_present",
			nodeName:     testNodeName,
			k8sObjects:   []runtime.Object{},
			expectResult: false,
		},
		{
			name:     "Pods_present_but_no_one_sts",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "test-ns-1",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment-1",
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-2",
						Namespace: "test-ns-2",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "DaemonSet",
								Name:       "test-daemonset-2",
							},
						},
					},
				},
			},
			expectResult: false,
		},
		{
			name:     "STS_pods_present",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "test-ns-1",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment-1",
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-2",
						Namespace: "test-ns-2",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "DaemonSet",
								Name:       "test-daemonset-2",
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-3",
						Namespace: "test-ns-3",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "StatefulSet",
								Name:       "test-sts-3",
							},
						},
					},
				},
			},
			expectResult: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			receivedResult := areSTSPodsPresentOnNode(fakeClientset, test.nodeName)

			switch test.expectResult {
			case false:
				if receivedResult {
					t.Fatalf("expected false but received true")
				} else {
					t.Logf("expected false and got it: %v", receivedResult)
				}
			case true:
				if !receivedResult {
					t.Fatalf("unexpected success: %v", receivedResult)
				}
			}
		})
	}
}

func TestSetAnnotationToNode(t *testing.T) {
	testCases := []struct {
		name            string
		nodeName        string
		annotationKey   string
		annotationValue string
		k8sObjects      []runtime.Object
		expectResult    bool
		expectSuccess   bool
	}{
		{
			name:            "Node_does_not_exist",
			nodeName:        testNodeName,
			annotationKey:   "test-annotation1",
			annotationValue: "test-value1",
			k8sObjects:      []runtime.Object{},
			expectResult:    false,
			expectSuccess:   false,
		},
		{
			name:            "Node_exist_and_annotations_empty",
			nodeName:        testNodeName,
			annotationKey:   "test-annotation2",
			annotationValue: "test-value2",
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
				},
			},
			expectResult:  true,
			expectSuccess: true,
		},
		{
			name:            "Node_exist_and_has_some_different_annotations",
			nodeName:        testNodeName,
			annotationKey:   "test-annotation3",
			annotationValue: "test-value3",
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
				},
			},
			expectResult:  true,
			expectSuccess: true,
		},
		{
			name:            "Node_exist_and_has_same_annotations_with_different_value",
			nodeName:        testNodeName,
			annotationKey:   "test-annotation4",
			annotationValue: "test-value4",
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
				},
			},
			expectResult:  true,
			expectSuccess: true,
		},
		{
			name:            "Node_exist_and_has_same_annotations_with_same_value",
			nodeName:        testNodeName,
			annotationKey:   "test-annotation5",
			annotationValue: "test-value5",
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
				},
			},
			expectResult:  true,
			expectSuccess: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			receivedResult := false
			err := setAnnotationToNode(fakeClientset, test.nodeName, test.annotationKey, test.annotationValue)
			if err == nil {
				node, _ := fakeClientset.CoreV1().Nodes().Get(
					context.TODO(),
					test.nodeName,
					metav1.GetOptions{},
				)
				if node.Annotations[test.annotationKey] == test.annotationValue {
					receivedResult = true
				}
			}
			switch test.expectSuccess {
			case false:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case true:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				} else if test.expectResult != receivedResult {
					t.Fatalf("expected node was annotated but it is not")
				}
			}
		})
	}
}

func TestWaitUntilDisruptionApproved(t *testing.T) {
	testCases := []struct {
		name          string
		nodeName      string
		k8sObjects    []runtime.Object
		expectSuccess bool
	}{
		{
			name:     "Node_exists_and_have_disruption_approved_annotation",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
						Annotations: map[string]string{
							"update.node.deckhouse.io/disruption-approved": "",
						},
					},
				},
			},
			expectSuccess: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			err := waitUntilDisruptionApproved(fakeClientset, test.nodeName)

			switch test.expectSuccess {
			case false:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case true:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
