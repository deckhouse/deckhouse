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
			_, isCurrentAgentPodGenerationDesired, err := checkAgentPodGeneration(fakeClientset, test.nodeName)
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
