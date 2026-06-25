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

package instance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestReconcileBashibleStatusPatchesStatusAndMessage(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "bashible-status"},
		Status: deckhousev1alpha2.InstanceStatus{
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status:  metav1.ConditionFalse,
				Reason:  "StepsFailed",
				Message: "boot step failed",
			}},
		},
	}
	c := newStatusClient(t, instance.DeepCopy())
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleStatus(context.Background(), instance))
	require.Equal(t, deckhousev1alpha2.BashibleStatusError, instance.Status.BashibleStatus)
	require.Equal(t, "bashible: boot step failed", instance.Status.Message)

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted))
	require.Equal(t, deckhousev1alpha2.BashibleStatusError, persisted.Status.BashibleStatus)
	require.Equal(t, "bashible: boot step failed", persisted.Status.Message)
}

func TestReconcileBashibleStatusNoChangeSkipsPatch(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "bashible-status-noop"},
		Status: deckhousev1alpha2.InstanceStatus{
			BashibleStatus: deckhousev1alpha2.BashibleStatusReady,
			Message:        "",
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	patchCalled := false
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(instance.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
				patchCalled = true
				return nil
			},
		}).
		Build()
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleStatus(context.Background(), instance))
	require.False(t, patchCalled)
}

func TestReconcileBashibleStatusClearsBootstrapStatus(t *testing.T) {
	t.Parallel()

	bootstrap := &deckhousev1alpha2.BootstrapStatus{LogsEndpoint: "http://logs", Description: "bootstrapping"}
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "bashible-clear-bootstrap"},
		Status: deckhousev1alpha2.InstanceStatus{
			BashibleStatus:  deckhousev1alpha2.BashibleStatusReady,
			BootstrapStatus: bootstrap,
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	c := newStatusClient(t, instance.DeepCopy())
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleStatus(context.Background(), instance))
	require.Nil(t, instance.Status.BootstrapStatus)
}

func TestHasBashibleReadyCondition(t *testing.T) {
	t.Parallel()

	require.True(t, hasBashibleReadyCondition([]deckhousev1alpha2.InstanceCondition{
		{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady},
	}))
	require.False(t, hasBashibleReadyCondition([]deckhousev1alpha2.InstanceCondition{
		{Type: deckhousev1alpha2.InstanceConditionTypeMachineReady},
	}))
	require.False(t, hasBashibleReadyCondition(nil))
}
