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

package main

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddInitContainerToPodPreservesNativeSidecarRestartPolicy(t *testing.T) {
	t.Parallel()

	restartPolicy := corev1.ContainerRestartPolicyAlways
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Subdomain: "test-subdomain",
			InitContainers: []corev1.Container{
				{
					Name:          "native-sidecar",
					Image:         "busybox:latest",
					RestartPolicy: &restartPolicy,
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "busybox:latest",
				},
			},
		},
	}

	result, err := addInitContainerToPod(context.Background(), nil, pod)
	if err != nil {
		t.Fatalf("add init container to pod: %v", err)
	}

	mutatedPod, ok := result.MutatedObject.(*corev1.Pod)
	if !ok {
		t.Fatalf("expected mutated object to be *corev1.Pod, got %T", result.MutatedObject)
	}

	if len(mutatedPod.Spec.InitContainers) != 2 {
		t.Fatalf("expected 2 init containers, got %d", len(mutatedPod.Spec.InitContainers))
	}

	nativeSidecar := mutatedPod.Spec.InitContainers[1]
	if nativeSidecar.Name != "native-sidecar" {
		t.Fatalf("expected native sidecar to remain after appended init container, got %q", nativeSidecar.Name)
	}
	if nativeSidecar.RestartPolicy == nil {
		t.Fatal("expected native sidecar restartPolicy to be preserved, got nil")
	}
	if *nativeSidecar.RestartPolicy != corev1.ContainerRestartPolicyAlways {
		t.Fatalf("expected native sidecar restartPolicy %q, got %q", corev1.ContainerRestartPolicyAlways, *nativeSidecar.RestartPolicy)
	}
}
