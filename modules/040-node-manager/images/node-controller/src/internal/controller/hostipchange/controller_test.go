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

package hostipchange

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl}}
}

func apiserverPod(name, hostIP, initialHostIP string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodecommon.MachineNamespace,
			Labels:    map[string]string{appLabelKey: appLabelValue},
		},
		Status: corev1.PodStatus{HostIP: hostIP},
	}
	if initialHostIP != "" {
		pod.Annotations = map[string]string{initialHostIPAnnotation: initialHostIP}
	}
	return pod
}

func reconcile(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func getPod(t *testing.T, r *Reconciler, name string) (*corev1.Pod, bool) {
	t.Helper()
	pod := &corev1.Pod{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, pod)
	if err != nil {
		return nil, false
	}
	return pod, true
}

// A pod without the annotation gets its current host IP recorded.
func TestReconcile_NoAnnotation_Recorded(t *testing.T) {
	r := newReconciler(t, apiserverPod("ba", "1.2.3.4", ""))
	reconcile(t, r, "ba")

	pod, ok := getPod(t, r, "ba")
	if !ok {
		t.Fatal("pod must survive the recording branch")
	}
	if got := pod.Annotations[initialHostIPAnnotation]; got != "1.2.3.4" {
		t.Fatalf("initial-host-ip = %q, want 1.2.3.4", got)
	}
}

// A pod whose recorded IP still matches the host IP is left untouched.
func TestReconcile_MatchingIP_Untouched(t *testing.T) {
	r := newReconciler(t, apiserverPod("ba", "1.2.3.4", "1.2.3.4"))
	reconcile(t, r, "ba")

	pod, ok := getPod(t, r, "ba")
	if !ok {
		t.Fatal("pod with matching IP must survive")
	}
	if got := pod.Annotations[initialHostIPAnnotation]; got != "1.2.3.4" {
		t.Fatalf("initial-host-ip = %q, want unchanged 1.2.3.4", got)
	}
}

// A pod whose recorded IP diverges from the live host IP is deleted.
func TestReconcile_ChangedIP_Deleted(t *testing.T) {
	r := newReconciler(t, apiserverPod("ba", "4.5.6.7", "1.2.3.4"))
	reconcile(t, r, "ba")

	if _, ok := getPod(t, r, "ba"); ok {
		t.Fatal("pod must be deleted when host IP changed")
	}
}

// A pod without a host IP (not scheduled) is neither annotated nor deleted.
func TestReconcile_EmptyHostIP_Untouched(t *testing.T) {
	r := newReconciler(t, apiserverPod("ba", "", "1.2.3.4"))
	reconcile(t, r, "ba")

	pod, ok := getPod(t, r, "ba")
	if !ok {
		t.Fatal("pod with empty host IP must survive")
	}
	if got := pod.Annotations[initialHostIPAnnotation]; got != "1.2.3.4" {
		t.Fatalf("initial-host-ip = %q, want unchanged 1.2.3.4", got)
	}
}

// A missing pod reconciles without error.
func TestReconcile_Missing_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "ba")
}
