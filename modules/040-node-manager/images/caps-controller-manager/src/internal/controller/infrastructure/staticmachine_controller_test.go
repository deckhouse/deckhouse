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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

const testNamespace = "d8-cloud-instance-manager"

func newStaticMachine(name string, nodeGroup string) *infrav1.StaticMachine {
	return &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: infrav1.StaticMachineSpec{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-group": nodeGroup},
			},
		},
	}
}

// A StaticInstance going back to Pending must only wake up the StaticMachines that could
// actually consume it. Enqueueing every non-ready StaticMachine turns one release into a
// cluster-wide reconcile burst.
func TestStaticInstanceToStaticMachineMapFuncMatchesLabelSelectorOnly(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	matching := newStaticMachine("matching", "worker")
	nonMatching := newStaticMachine("non-matching", "system")

	ready := newStaticMachine("ready", "worker")
	ready.Status.Ready = true

	provisioned := newStaticMachine("provisioned", "worker")
	provisioned.Status.Initialization.Provisioned = ptr.To(true)

	second := newStaticMachine("second", "worker")

	reconciler := &StaticMachineReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(matching, nonMatching, ready, provisioned, second).
			Build(),
	}

	instance := &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "static-instance",
			Labels: map[string]string{
				"node-group": "worker",
			},
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase: deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
			},
		},
	}

	mapFunc := reconciler.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))
	requests := mapFunc(context.Background(), instance)

	require.Len(t, requests, 2)

	names := make([]string, 0, len(requests))
	for _, request := range requests {
		require.Equal(t, testNamespace, request.Namespace)
		names = append(names, request.Name)
	}

	require.ElementsMatch(t, []string{"matching", "second"}, names)
}

// StaticInstances excluded from bootstrapping must not wake anyone up.
func TestStaticInstanceToStaticMachineMapFuncSkipsBootstrapDisabledInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	reconciler := &StaticMachineReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(newStaticMachine("matching", "worker")).
			Build(),
	}

	instance := &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "static-instance",
			Labels: map[string]string{
				"node-group":                        "worker",
				"node.deckhouse.io/allow-bootstrap": "false",
			},
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase: deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
			},
		},
	}

	mapFunc := reconciler.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))

	require.Empty(t, mapFunc(context.Background(), instance))
}
