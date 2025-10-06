/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testNodeName = "testcluster-worker-02334ee2-7694f-h9zgk"
)

func TestGetPodCIDR(t *testing.T) {
	testCases := []struct {
		name             string
		nodeName         string
		k8sObjects       []runtime.Object
		expectPodCIDRStr string
		expectSuccess    bool
	}{
		{
			name:             "Node_doesnt_exist",
			nodeName:         testNodeName,
			k8sObjects:       []runtime.Object{},
			expectPodCIDRStr: "10.111.1.0/24",
			expectSuccess:    false,
		},
		{
			name:     "Node_exist_but_PodCIDR_doesnt",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
					Spec: v1.NodeSpec{
						PodCIDR: "",
					},
				},
			},
			expectPodCIDRStr: "10.111.1.0/24",
			expectSuccess:    false,
		},
		{
			name:     "Node_and_PodCIDR_exist",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNodeName,
					},
					Spec: v1.NodeSpec{
						PodCIDR: "10.111.1.0/24",
					},
				},
			},
			expectPodCIDRStr: "10.111.1.0/24",
			expectSuccess:    true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			nldCCTest := &ConnectionsCleaner{
				kubeClient:       fakeClientset,
				checkInterval:    scanInterval,
				listenAddress:    listenAddress,
				dstPort:          nldDstPort,
				nameSpace:        nldNS,
				podLabelSelector: nldLabelSelector,
				nodeName:         testNodeName,
			}
			ctx, cancel := context.WithCancel(context.Background())
			PodCIDR, err := nldCCTest.getPodCIDR(ctx)
			cancel()

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
				} else {
					if test.expectPodCIDRStr != PodCIDR.String() {
						t.Fatalf("podCIDR is not equal to the expected value")
					}
				}
			}
		})
	}
}

func TestGetNLDPodNameAndIPByNodeName(t *testing.T) {
	testCases := []struct {
		name           string
		nodeName       string
		k8sObjects     []runtime.Object
		expectPodIPStr string
		expectPodName  string
		expectSuccess  bool
	}{
		{
			name:           "Pod_does_not_exist",
			nodeName:       testNodeName,
			k8sObjects:     []runtime.Object{},
			expectPodIPStr: "10.111.1.42",
			expectPodName:  "node-local-dns-12345",
			expectSuccess:  false,
		},
		{
			name:     "Pod_exist_but_doesn't_have_IP",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-local-dns-12345",
						Namespace: nldNS,
						Labels: map[string]string{
							"app": "node-local-dns",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
				},
			},
			expectPodIPStr: "<nil>",
			expectPodName:  "node-local-dns-12345",
			expectSuccess:  true,
		},
		{
			name:     "Pod_exist_and_have_IP",
			nodeName: testNodeName,
			k8sObjects: []runtime.Object{
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-local-dns-12345",
						Namespace: nldNS,
						Labels: map[string]string{
							"app": "node-local-dns",
						},
					},
					Spec: v1.PodSpec{
						NodeName: testNodeName,
					},
					Status: v1.PodStatus{
						PodIP: "10.111.1.42",
					},
				},
			},
			expectPodIPStr: "10.111.1.42",
			expectPodName:  "node-local-dns-12345",
			expectSuccess:  true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset(test.k8sObjects...)
			nldCCTest := &ConnectionsCleaner{
				kubeClient:       fakeClientset,
				checkInterval:    scanInterval,
				listenAddress:    listenAddress,
				dstPort:          nldDstPort,
				nameSpace:        nldNS,
				podLabelSelector: nldLabelSelector,
				nodeName:         testNodeName,
			}
			ctx, cancel := context.WithCancel(context.Background())
			podName, podIP, err := nldCCTest.getNLDPodNameAndIPByNodeName(ctx)
			cancel()

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
				} else {
					if test.expectPodName != podName {
						t.Fatalf("podName is not equal to the expected value")
					} else if test.expectPodIPStr != podIP.String() {
						t.Fatalf("podIP(%v) is not equal to the expected value(%v).", podIP.String(), test.expectPodIPStr)
					}
				}
			}
		})
	}
}
