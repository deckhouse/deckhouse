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

package drain

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWaitForDeleteDoesNotTreatPodLookupErrorAsDeletion(t *testing.T) {
	lookupErr := errors.New("pod lookup failed")
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-pod",
			UID:       "original-uid",
		},
	}

	var finishErr error
	remainingPods, err := waitForDelete(waitForDeleteParams{
		ctx:      context.Background(),
		pods:     []corev1.Pod{pod},
		interval: time.Millisecond,
		timeout:  time.Second,
		getPodFn: func(namespace, name string) (*corev1.Pod, error) {
			if namespace != pod.Namespace || name != pod.Name {
				t.Fatalf("getPodFn called for an unexpected pod: %s/%s", namespace, name)
			}

			// The typed Kubernetes client returns a non-nil, empty result object
			// together with an error. The empty UID must not make the original pod
			// look deleted when the lookup itself failed.
			return &corev1.Pod{}, lookupErr
		},
		onFinishFn: func(_ *corev1.Pod, _ bool, err error) {
			finishErr = err
		},
		out: io.Discard,
	})

	if !errors.Is(err, lookupErr) {
		t.Fatalf("waitForDelete() error = %v, want %v", err, lookupErr)
	}
	if !errors.Is(finishErr, lookupErr) {
		t.Fatalf("onFinishFn error = %v, want %v", finishErr, lookupErr)
	}
	if len(remainingPods) != 1 {
		t.Fatalf("waitForDelete() returned %d remaining pods, want 1", len(remainingPods))
	}
	if remainingPods[0].UID != pod.UID {
		t.Fatalf("remaining pod UID = %q, want %q", remainingPods[0].UID, pod.UID)
	}
}
