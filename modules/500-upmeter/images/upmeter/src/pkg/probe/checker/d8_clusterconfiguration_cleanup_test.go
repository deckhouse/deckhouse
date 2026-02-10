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

package checker

import (
	"context"
	"io"
	"testing"
	"time"

	kube "github.com/flant/kube-client/client"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"d8.io/upmeter/pkg/probe/run"
)

func TestCleanupExtraHookProbesChecker_DeletesWhenHashNotFromAgentPods(t *testing.T) {
	ctx := context.TODO()
	fakeClient := kube.NewFake(nil)
	access := NewFake(fakeClient)

	gvr := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "upmeterhookprobes",
	}
	dynamicClient := fakeClient.Dynamic().Resource(gvr)

	if err := createAgentPod(ctx, fakeClient, "agent-1", "master-1"); err != nil {
		t.Fatalf("create agent pod 1: %v", err)
	}
	if err := createAgentPod(ctx, fakeClient, "agent-2", "master-2"); err != nil {
		t.Fatalf("create agent pod 2: %v", err)
	}

	oldTime := time.Now().Add(-10 * time.Minute)

	expected := []string{
		run.NodeNameHash("master-1"),
		run.NodeNameHash("master-2"),
	}
	for _, name := range expected {
		if err := createHookProbe(ctx, dynamicClient, name, oldTime); err != nil {
			t.Fatalf("create expected hook probe %q: %v", name, err)
		}
	}

	extraName := run.NodeNameHash("master-3")
	if err := createHookProbe(ctx, dynamicClient, extraName, oldTime); err != nil {
		t.Fatalf("create extra hook probe: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	checker := &cleanupExtraHookProbesChecker{
		access:        access,
		dynamicClient: dynamicClient,
		logger:        logrus.NewEntry(logger),
	}

	if err := checker.Check(); err != nil {
		t.Fatalf("cleanup checker error: %v", err)
	}

	list, err := dynamicClient.List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		t.Fatalf("list hook probes: %v", err)
	}

	got := make(map[string]struct{})
	for i := range list.Items {
		got[list.Items[i].GetName()] = struct{}{}
	}

	for _, name := range expected {
		if _, ok := got[name]; !ok {
			t.Fatalf("expected hook probe %q to remain", name)
		}
	}
	if _, ok := got[extraName]; ok {
		t.Fatalf("expected extra hook probe %q to be deleted", extraName)
	}
}

func createAgentPod(ctx context.Context, client kube.Client, name, nodeName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "d8-upmeter",
			Labels: map[string]string{
				"app": "upmeter-agent",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
	_, err := client.CoreV1().Pods("d8-upmeter").Create(ctx, pod, metav1.CreateOptions{})
	return err
}

func createHookProbe(ctx context.Context, client dynamic.ResourceInterface, name string, createdAt time.Time) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("deckhouse.io/v1")
	obj.SetKind("UpmeterHookProbe")
	obj.SetName(name)
	obj.SetLabels(map[string]string{
		"heritage": "upmeter",
	})
	obj.SetCreationTimestamp(metav1.NewTime(createdAt))

	_, err := client.Create(ctx, obj, metav1.CreateOptions{})
	return err
}
