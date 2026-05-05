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

package instance_controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func TestReconcileMachineStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		machine                 client.Object
		instance                *deckhousev1alpha2.Instance
		wantPhase               deckhousev1alpha2.InstancePhase
		wantMachineStatus       string
		wantMachineReadyStatus  metav1.ConditionStatus
		wantMachineReadyReason  string
		wantMachineReadyMessage string
	}{
		{
			name: "capi infrastructure wait sync",
			machine: capiMachineWithStatus("capi-pending", capiv1beta2.MachineStatus{
				Phase: string(capiv1beta2.MachinePhasePending),
				Conditions: []metav1.Condition{{
					Type:               capiv1beta2.InfrastructureReadyCondition,
					Status:             metav1.ConditionFalse,
					Reason:             "WaitingForInfrastructure",
					LastTransitionTime: metav1.Now(),
				}},
			}),
			instance: existingInstance("capi-pending", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capiv1beta2.GroupVersion.String(),
					Name:       "capi-pending",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseUnknown),
			wantPhase:               deckhousev1alpha2.InstancePhasePending,
			wantMachineStatus:       "Progressing",
			wantMachineReadyStatus:  metav1.ConditionFalse,
			wantMachineReadyReason:  "WaitingForInfrastructure",
			wantMachineReadyMessage: "Waiting for infrastructure",
		},
		{
			name: "capi deleting drain blocked sync",
			machine: capiMachineWithStatus("capi-deleting", capiv1beta2.MachineStatus{
				Phase: string(capiv1beta2.MachinePhaseDeleting),
				Conditions: []metav1.Condition{{
					Type:               capiv1beta2.DeletingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             capiv1beta2.MachineDeletingDrainingNodeReason,
					Message:            "cannot evict pod because disruption budget",
					LastTransitionTime: metav1.Now(),
				}},
			}),
			instance: existingInstance("capi-deleting", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capiv1beta2.GroupVersion.String(),
					Name:       "capi-deleting",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseRunning),
			wantPhase:               deckhousev1alpha2.InstancePhaseTerminating,
			wantMachineStatus:       "Blocked",
			wantMachineReadyStatus:  metav1.ConditionFalse,
			wantMachineReadyReason:  capiv1beta2.MachineDeletingDrainingNodeReason,
			wantMachineReadyMessage: "cannot evict pod because disruption budget",
		},
		{
			name: "mcm running ready sync",
			machine: mcmMachineWithStatus("mcm-running", mcmv1alpha1.MachineStatus{
				CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning},
				LastOperation: mcmv1alpha1.LastOperation{
					State:       mcmv1alpha1.MachineStateSuccessful,
					Description: "machine   is ready",
				},
			}),
			instance: existingInstance("mcm-running", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "mcm-running",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseUnknown),
			wantPhase:               deckhousev1alpha2.InstancePhaseRunning,
			wantMachineStatus:       "Ready",
			wantMachineReadyStatus:  metav1.ConditionTrue,
			wantMachineReadyReason:  "Ready",
			wantMachineReadyMessage: "machine is ready",
		},
		{
			name: "mcm terminating drain blocked sync",
			machine: mcmMachineWithStatus("mcm-terminating", mcmv1alpha1.MachineStatus{
				CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineTerminating},
				LastOperation: mcmv1alpha1.LastOperation{
					State:       mcmv1alpha1.MachineStateFailed,
					Description: "drain   failed due to disruption budget",
				},
			}),
			instance: existingInstance("mcm-terminating", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "mcm-terminating",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseRunning),
			wantPhase:               deckhousev1alpha2.InstancePhaseTerminating,
			wantMachineStatus:       "Blocked",
			wantMachineReadyStatus:  metav1.ConditionFalse,
			wantMachineReadyReason:  "DeleteFailed",
			wantMachineReadyMessage: "drain failed due to disruption budget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
			controller, k8sClient := newTestInstanceController(t, nil, tt.machine, tt.instance)

			result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tt.instance.Name}})
			require.NoError(t, err)
			require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

			instance := &deckhousev1alpha2.Instance{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tt.instance.Name}, instance)
			require.NoError(t, err)
			require.Equal(t, tt.wantPhase, instance.Status.Phase)
			require.Equal(t, tt.wantMachineStatus, instance.Status.MachineStatus)

			condition := requireMachineReadyCondition(t, instance.Status.Conditions)
			require.Equal(t, tt.wantMachineReadyStatus, condition.Status)
			require.Equal(t, tt.wantMachineReadyReason, condition.Reason)
			require.Equal(t, tt.wantMachineReadyMessage, condition.Message)
		})
	}
}

func TestReconcileBashibleStatus(t *testing.T) {
	t.Parallel()

	type bashibleExpectation struct {
		wantBashibleStatus deckhousev1alpha2.BashibleStatus
		wantMessage        string
		wantMachineReady   metav1.ConditionStatus
		wantMachineReason  string
		wantMachineMessage string
	}

	tests := []struct {
		name       string
		machine    client.Object
		instance   *deckhousev1alpha2.Instance
		wantFirst  bashibleExpectation
		wantSecond *struct {
			updatedMachineStatus mcmv1alpha1.MachineStatus
			want                 bashibleExpectation
		}
	}{
		{
			name:    "bashible ready true produces ready status",
			machine: mcmMachineWithStatus("ready", mcmv1alpha1.MachineStatus{CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning}}),
			instance: instanceWithConditions("ready", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "ready",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseRunning,
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionTrue,
					Message: "last successful step",
				},
			),
			wantFirst: bashibleExpectation{
				wantBashibleStatus: deckhousev1alpha2.BashibleStatusReady,
				wantMessage:        "bashible: last successful step",
				wantMachineReady:   metav1.ConditionTrue,
				wantMachineReason:  "Ready",
				wantMachineMessage: "",
			},
		},
		{
			name:    "waiting approval timeout produces waiting approval status",
			machine: mcmMachineWithStatus("waiting-approval", mcmv1alpha1.MachineStatus{CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning}}),
			instance: instanceWithConditions("waiting-approval", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "waiting-approval",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseRunning,
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionTrue,
					Message: "last successful step",
				},
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status:  metav1.ConditionTrue,
					Reason:  deckhousev1alpha2.InstanceConditionReasonUpdateApprovalTimeout,
					Message: "waiting for approval",
				},
			),
			wantFirst: bashibleExpectation{
				wantBashibleStatus: deckhousev1alpha2.BashibleStatusWaitingApproval,
				wantMessage:        "bashible: waiting for approval",
				wantMachineReady:   metav1.ConditionTrue,
				wantMachineReason:  "Ready",
				wantMachineMessage: "",
			},
		},
		{
			name: "machine condition keeps message priority over bashible",
			machine: capiMachineWithStatus("machine-priority", capiv1beta2.MachineStatus{
				Phase: string(capiv1beta2.MachinePhasePending),
				Conditions: []metav1.Condition{{
					Type:               capiv1beta2.InfrastructureReadyCondition,
					Status:             metav1.ConditionFalse,
					Reason:             "WaitingForInfrastructure",
					LastTransitionTime: metav1.Now(),
				}},
			}),
			instance: instanceWithConditions("machine-priority", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capiv1beta2.GroupVersion.String(),
					Name:       "machine-priority",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhasePending,
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:  metav1.ConditionFalse,
					Reason:  "WaitingForInfrastructure",
					Message: "Waiting for infrastructure",
				},
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionFalse,
					Message: "bashible failed",
				},
			),
			wantFirst: bashibleExpectation{
				wantBashibleStatus: deckhousev1alpha2.BashibleStatusError,
				wantMessage:        "machine: Waiting for infrastructure",
				wantMachineReady:   metav1.ConditionFalse,
				wantMachineReason:  "WaitingForInfrastructure",
				wantMachineMessage: "Waiting for infrastructure",
			},
		},
		{
			name: "machine-derived message refreshes stale machine message after second reconcile",
			machine: mcmMachineWithStatus("mcm-message-update", mcmv1alpha1.MachineStatus{
				CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineTerminating},
				LastOperation: mcmv1alpha1.LastOperation{
					State:          mcmv1alpha1.MachineStateFailed,
					Description:    "drain failed due to disruption budget",
					LastUpdateTime: metav1.Now(),
				},
			}),
			instance: instanceWithConditions("mcm-message-update", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "mcm-message-update",
					Namespace:  machine.MachineNamespace,
				},
			}, deckhousev1alpha2.InstancePhaseTerminating,
				deckhousev1alpha2.InstanceCondition{
					Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:  metav1.ConditionFalse,
					Reason:  "PreviousReason",
					Message: "outdated machine message",
				},
			),
			wantFirst: bashibleExpectation{
				wantBashibleStatus: deckhousev1alpha2.BashibleStatusUnknown,
				wantMessage:        "machine: outdated machine message",
				wantMachineReady:   metav1.ConditionFalse,
				wantMachineReason:  "DeleteFailed",
				wantMachineMessage: "drain failed due to disruption budget",
			},
			wantSecond: &struct {
				updatedMachineStatus mcmv1alpha1.MachineStatus
				want                 bashibleExpectation
			}{
				updatedMachineStatus: mcmv1alpha1.MachineStatus{
					CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning},
					LastOperation: mcmv1alpha1.LastOperation{
						State:          mcmv1alpha1.MachineStateSuccessful,
						Description:    "machine is ready",
						LastUpdateTime: metav1.Now(),
					},
				},
				want: bashibleExpectation{
					wantBashibleStatus: deckhousev1alpha2.BashibleStatusUnknown,
					wantMessage:        "machine: drain failed due to disruption budget",
					wantMachineReady:   metav1.ConditionTrue,
					wantMachineReason:  "Ready",
					wantMachineMessage: "machine is ready",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
			var machineClient *switchingMCMGetClient
			clientFactory := func(base client.WithWatch) client.Client {
				if tt.wantSecond == nil {
					return base
				}

				machineClient = &switchingMCMGetClient{
					Client: base,
					name:   types.NamespacedName{Namespace: machine.MachineNamespace, Name: tt.instance.Name},
				}
				return machineClient
			}
			controller, k8sClient := newTestInstanceController(t, clientFactory, tt.machine, tt.instance)

			result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tt.instance.Name}})
			require.NoError(t, err)
			require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

			instance := &deckhousev1alpha2.Instance{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tt.instance.Name}, instance)
			require.NoError(t, err)
			require.Equal(t, tt.wantFirst.wantBashibleStatus, instance.Status.BashibleStatus)
			require.Equal(t, tt.wantFirst.wantMessage, instance.Status.Message)

			condition := requireMachineReadyCondition(t, instance.Status.Conditions)
			require.Equal(t, tt.wantFirst.wantMachineReady, condition.Status)
			require.Equal(t, tt.wantFirst.wantMachineReason, condition.Reason)
			require.Equal(t, tt.wantFirst.wantMachineMessage, condition.Message)

			if tt.wantSecond != nil {
				machineClient.SetStatus(tt.wantSecond.updatedMachineStatus)

				result, err = controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tt.instance.Name}})
				require.NoError(t, err)
				require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

				err = k8sClient.Get(ctx, types.NamespacedName{Name: tt.instance.Name}, instance)
				require.NoError(t, err)
				require.Equal(t, tt.wantSecond.want.wantBashibleStatus, instance.Status.BashibleStatus)
				require.Equal(t, tt.wantSecond.want.wantMessage, instance.Status.Message)

				condition = requireMachineReadyCondition(t, instance.Status.Conditions)
				require.Equal(t, tt.wantSecond.want.wantMachineReady, condition.Status)
				require.Equal(t, tt.wantSecond.want.wantMachineReason, condition.Reason)
				require.Equal(t, tt.wantSecond.want.wantMachineMessage, condition.Message)
			}
		})
	}
}

func TestReconcileConflictRequeues(t *testing.T) {
	t.Parallel()

	instance := existingInstanceWithFinalizer("conflict-machine-status", deckhousev1alpha2.InstanceSpec{
		MachineRef: &deckhousev1alpha2.MachineRef{
			Kind:       "Machine",
			APIVersion: capiv1beta2.GroupVersion.String(),
			Name:       "conflict-machine-status",
			Namespace:  machine.MachineNamespace,
		},
	}, deckhousev1alpha2.InstancePhaseUnknown)

	conflictErr := apierrors.NewConflict(
		schema.GroupResource{Group: deckhousev1alpha2.GroupVersion.Group, Resource: "instances"},
		instance.Name,
		fmt.Errorf("simulated conflict"),
	)

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
	controller, _ := newTestInstanceController(t, func(base client.WithWatch) client.Client {
		return conflictOnInstanceStatusPatchClient{Client: base, err: conflictErr}
	}, capiMachineWithStatus("conflict-machine-status", capiv1beta2.MachineStatus{
		Phase: string(capiv1beta2.MachinePhaseRunning),
	}), instance)

	result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: instance.Name}})
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{Requeue: true}, result)
}

func TestReconcileDeletingInstanceSyncsMachineStatus(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	instance := existingInstanceWithFinalizer("deleting-instance", deckhousev1alpha2.InstanceSpec{
		MachineRef: &deckhousev1alpha2.MachineRef{
			Kind:       "Machine",
			APIVersion: capiv1beta2.GroupVersion.String(),
			Name:       "deleting-instance",
			Namespace:  machine.MachineNamespace,
		},
	}, deckhousev1alpha2.InstancePhaseRunning)
	instance.DeletionTimestamp = &now
	instance.Status.BashibleStatus = deckhousev1alpha2.BashibleStatusReady
	instance.Status.Message = "bashible: last successful step"
	instance.Status.Conditions = []deckhousev1alpha2.InstanceCondition{{
		Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
		Status:  metav1.ConditionTrue,
		Message: "last successful step",
	}}

	machineObj := capiMachineWithStatus("deleting-instance", capiv1beta2.MachineStatus{
		Phase: string(capiv1beta2.MachinePhaseDeleting),
		Conditions: []metav1.Condition{{
			Type:               capiv1beta2.DeletingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             capiv1beta2.MachineDeletingDrainingNodeReason,
			Message:            "cannot evict pod because disruption budget",
			LastTransitionTime: now,
		}},
	})
	machineObj.DeletionTimestamp = &now
	machineObj.Finalizers = []string{"machine.cluster.x-k8s.io"}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
	controller, k8sClient := newTestInstanceController(t, nil, machineObj, instance)

	result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: instance.Name}})
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

	persisted := &deckhousev1alpha2.Instance{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: instance.Name}, persisted)
	require.NoError(t, err)
	require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
	require.Equal(t, deckhousev1alpha2.InstancePhaseTerminating, persisted.Status.Phase)
	require.Equal(t, string(machine.StatusBlocked), persisted.Status.MachineStatus)
	require.Equal(t, deckhousev1alpha2.BashibleStatusReady, persisted.Status.BashibleStatus)
	require.Equal(t, "machine: cannot evict pod because disruption budget", persisted.Status.Message)

	condition := requireMachineReadyCondition(t, persisted.Status.Conditions)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, capiv1beta2.MachineDeletingDrainingNodeReason, condition.Reason)
	require.Equal(t, "cannot evict pod because disruption budget", condition.Message)
}

func TestReconcileCreateFromSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		targetName                string
		objects                   []client.Object
		clientFactory             func(client.WithWatch) client.Client
		wantResult                ctrl.Result
		wantErr                   string
		wantInstance              *deckhousev1alpha2.Instance
		wantInstancePhase         deckhousev1alpha2.InstancePhase
		wantInstanceStatusMessage string
		verifyMachineStatusEmpty  bool
		expectNodeRefNameEmpty    bool
	}{
		{
			name:       "capi machine present",
			targetName: "node-a",
			objects: []client.Object{
				capiMachineWithStatus("node-a", capiv1beta2.MachineStatus{
					Phase:   string(capiv1beta2.MachinePhaseProvisioned),
					NodeRef: capiv1beta2.MachineNodeReference{Name: "capi-node-a"},
					Conditions: []metav1.Condition{{
						Type:               capiv1beta2.InfrastructureReadyCondition,
						Status:             metav1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
					}},
				}),
				mcmMachineWithStatus("node-a", mcmv1alpha1.MachineStatus{
					Node:          "node-a",
					CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning},
					LastOperation: mcmv1alpha1.LastOperation{
						State:       mcmv1alpha1.MachineStateSuccessful,
						Description: "ready",
					},
				}),
				staticNode("node-a"),
			},
			wantResult: ctrl.Result{},
			wantInstance: existingInstance("node-a", deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "capi-node-a"},
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capiv1beta2.GroupVersion.String(),
					Name:       "node-a",
					Namespace:  machine.MachineNamespace,
				},
			}, ""),
			verifyMachineStatusEmpty: true,
		},
		{
			name:       "capi machine without node ref",
			targetName: "node-no-node",
			objects: []client.Object{
				capiMachineWithStatus("node-no-node", capiv1beta2.MachineStatus{
					Phase: string(capiv1beta2.MachinePhasePending),
					Conditions: []metav1.Condition{{
						Type:               capiv1beta2.InfrastructureReadyCondition,
						Status:             metav1.ConditionFalse,
						Reason:             "Pending",
						LastTransitionTime: metav1.Now(),
					}},
				}),
			},
			wantResult: ctrl.Result{},
			wantInstance: existingInstance("node-no-node", deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capiv1beta2.GroupVersion.String(),
					Name:       "node-no-node",
					Namespace:  machine.MachineNamespace,
				},
			}, ""),
			verifyMachineStatusEmpty: true,
			expectNodeRefNameEmpty:   true,
		},
		{
			name:       "capi absent mcm present",
			targetName: "node-b",
			objects: []client.Object{
				mcmMachineWithStatus("node-b", mcmv1alpha1.MachineStatus{
					Node:          "node-b",
					CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachinePending},
					LastOperation: mcmv1alpha1.LastOperation{
						State:       mcmv1alpha1.MachineStateProcessing,
						Description: "provisioning",
					},
				}),
				staticNode("node-b"),
			},
			wantResult: ctrl.Result{},
			wantInstance: existingInstance("node-b", deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "node-b"},
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
					Name:       "node-b",
					Namespace:  machine.MachineNamespace,
				},
			}, ""),
			verifyMachineStatusEmpty: true,
		},
		{
			name:       "both absent static node present",
			targetName: "node-c",
			objects: []client.Object{
				staticNode("node-c"),
			},
			wantResult: ctrl.Result{},
			wantInstance: existingInstance("node-c", deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "node-c"},
			}, deckhousev1alpha2.InstancePhaseRunning),
			wantInstancePhase: deckhousev1alpha2.InstancePhaseRunning,
		},
		{
			name:       "both absent non static node present",
			targetName: "node-d",
			objects: []client.Object{
				nonStaticNode("node-d"),
			},
			wantResult: ctrl.Result{},
		},
		{
			name:       "all absent",
			targetName: "node-missing",
			wantResult: ctrl.Result{},
		},
		{
			name: "capi get error propagation",
			objects: []client.Object{
				mcmMachineWithStatus("node-e", mcmv1alpha1.MachineStatus{}),
				staticNode("node-e"),
			},
			targetName: "node-e",
			clientFactory: func(base client.WithWatch) client.Client {
				return errOnCAPIGetClient{
					Client: base,
					name:   types.NamespacedName{Namespace: machine.MachineNamespace, Name: "node-e"},
					err:    fmt.Errorf("capi get boom"),
				}
			},
			wantErr: `get capi machine "node-e": capi get boom`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
			controller, k8sClient := newTestInstanceController(t, tt.clientFactory, tt.objects...)
			assertInstanceMissing(t, ctx, k8sClient, tt.targetName)

			result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tt.targetName}})
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.Equal(t, ctrl.Result{}, result)
				assertInstanceMissing(t, ctx, k8sClient, tt.targetName)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantResult, result)

			if tt.wantInstance == nil {
				assertInstanceMissing(t, ctx, k8sClient, tt.targetName)
				return
			}

			instance := &deckhousev1alpha2.Instance{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tt.wantInstance.Name}, instance)
			require.NoError(t, err)
			require.Equal(t, tt.wantInstance.Spec, instance.Spec)
			require.Equal(t, tt.wantInstanceStatusMessage, instance.Status.Message)
			if tt.expectNodeRefNameEmpty {
				require.Empty(t, instance.Spec.NodeRef.Name)
			}
			if tt.verifyMachineStatusEmpty {
				require.Empty(t, instance.Status.Phase)
				require.Empty(t, instance.Status.MachineStatus)
				assertMachineReadyConditionAbsent(t, instance.Status.Conditions)
			}
			if tt.wantInstancePhase != "" {
				require.Equal(t, tt.wantInstancePhase, instance.Status.Phase)
			}
		})
	}
}

func TestReconcileExistingInstanceBindsMachineSource(t *testing.T) {
	t.Parallel()

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
	classReference := &deckhousev1alpha2.ClassReference{
		Kind: "DVPInstanceClass",
		Name: "worker",
	}
	instance := existingInstanceWithFinalizer("worker-a", deckhousev1alpha2.InstanceSpec{
		NodeRef:        deckhousev1alpha2.NodeRef{Name: "worker-a"},
		ClassReference: classReference,
	}, deckhousev1alpha2.InstancePhaseRunning)
	controller, k8sClient := newTestInstanceController(t, nil,
		instance,
		capiMachineWithStatus("worker-a", capiv1beta2.MachineStatus{
			Phase:   string(capiv1beta2.MachinePhaseRunning),
			NodeRef: capiv1beta2.MachineNodeReference{Name: "worker-a"},
		}),
	)

	result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-a"}})
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

	persisted := &deckhousev1alpha2.Instance{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "worker-a"}, persisted)
	require.NoError(t, err)
	require.Equal(t, deckhousev1alpha2.NodeRef{Name: "worker-a"}, persisted.Spec.NodeRef)
	require.Equal(t, classReference, persisted.Spec.ClassReference)
	require.Equal(t, &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: capiv1beta2.GroupVersion.String(),
		Name:       "worker-a",
		Namespace:  machine.MachineNamespace,
	}, persisted.Spec.MachineRef)
}

func TestReconcileSourceExistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		instance       *deckhousev1alpha2.Instance
		initialObjects []client.Object
		clientFactory  func(client.WithWatch) client.Client
		wantErr        string
	}{
		{
			name: "full reconcile keeps node-backed instance when source lookup succeeds",
			instance: existingInstanceWithFinalizer("node-source-live", deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "static-node"},
			}, deckhousev1alpha2.InstancePhaseRunning),
			initialObjects: []client.Object{
				staticNode("static-node"),
			},
		},
		{
			name: "full reconcile propagates node lookup error from source existence step",
			instance: existingInstanceWithFinalizer("node-source-error", deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "broken-node"},
			}, deckhousev1alpha2.InstancePhaseRunning),
			clientFactory: func(base client.WithWatch) client.Client {
				return errOnNodeGetClient{
					Client: base,
					name:   types.NamespacedName{Name: "broken-node"},
					err:    fmt.Errorf("node get boom"),
				}
			},
			wantErr: "get node \"broken-node\": node get boom",
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
			objects := make([]client.Object, 0, 1+len(tc.initialObjects))
			objects = append(objects, tc.instance.DeepCopy())
			objects = append(objects, tc.initialObjects...)
			controller, k8sClient := newTestInstanceController(t, tc.clientFactory, objects...)

			result, err := controller.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.instance.Name}})
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				require.Equal(t, ctrl.Result{}, result)

				persisted := &deckhousev1alpha2.Instance{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: tc.instance.Name}, persisted)
				require.NoError(t, err)
				require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
				return
			}

			require.NoError(t, err)
			require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

			persisted := &deckhousev1alpha2.Instance{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tc.instance.Name}, persisted)
			require.NoError(t, err)
			require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
			require.Equal(t, tc.instance.Status.Phase, persisted.Status.Phase)
		})
	}
}

func newTestInstanceController(
	t *testing.T,
	clientFactory func(client.WithWatch) client.Client,
	objects ...client.Object,
) (*InstanceController, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))
	require.NoError(t, capiv1beta2.AddToScheme(scheme))
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}, &capiv1beta2.Machine{}, &mcmv1alpha1.Machine{}).
		WithObjects(objects...).
		Build()
	controllerClient := client.Client(k8sClient)
	if clientFactory != nil {
		controllerClient = clientFactory(k8sClient)
	}

	controller := &InstanceController{Base: dynctrl.Base{Client: controllerClient}}
	require.NoError(t, controller.Setup(nil))

	return controller, k8sClient
}

func assertInstanceMissing(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()

	instance := &deckhousev1alpha2.Instance{}
	err := c.Get(ctx, types.NamespacedName{Name: name}, instance)
	require.True(t, apierrors.IsNotFound(err), "expected instance %q to be absent, got err=%v", name, err)
}

func assertMachineReadyConditionAbsent(t *testing.T, conditions []deckhousev1alpha2.InstanceCondition) {
	t.Helper()
	for _, condition := range conditions {
		if condition.Type == deckhousev1alpha2.InstanceConditionTypeMachineReady {
			t.Fatalf("did not expect MachineReady condition")
		}
	}
}

func requireMachineReadyCondition(t *testing.T, conditions []deckhousev1alpha2.InstanceCondition) deckhousev1alpha2.InstanceCondition {
	t.Helper()

	for _, condition := range conditions {
		if condition.Type == deckhousev1alpha2.InstanceConditionTypeMachineReady {
			return condition
		}
	}

	t.Fatalf("expected to find %q condition", deckhousev1alpha2.InstanceConditionTypeMachineReady)
	return deckhousev1alpha2.InstanceCondition{}
}

func capiMachineWithStatus(name string, status capiv1beta2.MachineStatus) *capiv1beta2.Machine {
	return &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machine.MachineNamespace,
		},
		Status: status,
	}
}

func mcmMachineWithStatus(name string, status mcmv1alpha1.MachineStatus) *mcmv1alpha1.Machine {
	return &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machine.MachineNamespace,
		},
		Status: status,
	}
}

func existingInstance(name string, spec deckhousev1alpha2.InstanceSpec, phase deckhousev1alpha2.InstancePhase) *deckhousev1alpha2.Instance {
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	if phase != "" {
		instance.Status.Phase = phase
	}
	return instance
}

func existingInstanceWithFinalizer(name string, spec deckhousev1alpha2.InstanceSpec, phase deckhousev1alpha2.InstancePhase) *deckhousev1alpha2.Instance {
	instance := existingInstance(name, spec, phase)
	instance.Finalizers = append(instance.Finalizers, instancecommon.InstanceControllerFinalizer)
	return instance
}

func instanceWithConditions(name string, spec deckhousev1alpha2.InstanceSpec, phase deckhousev1alpha2.InstancePhase, conditions ...deckhousev1alpha2.InstanceCondition) *deckhousev1alpha2.Instance {
	instance := existingInstance(name, spec, phase)
	instance.Status.Conditions = append(instance.Status.Conditions, conditions...)
	return instance
}

func capiMachine(name, nodeName string) *capiv1beta2.Machine {
	return capiMachineWithStatus(name, capiv1beta2.MachineStatus{
		NodeRef: capiv1beta2.MachineNodeReference{Name: nodeName},
	})
}

func mcmMachine(name, nodeName string) *mcmv1alpha1.Machine {
	return mcmMachineWithStatus(name, mcmv1alpha1.MachineStatus{
		Node: nodeName,
	})
}

func staticNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.deckhouse.io/type": "Static",
			},
		},
	}
}

func nonStaticNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.deckhouse.io/type": "CloudEphemeral",
			},
		},
	}
}

type errOnCAPIGetClient struct {
	client.Client
	name types.NamespacedName
	err  error
}

func (c errOnCAPIGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*capiv1beta2.Machine); ok && key == c.name {
		return c.err
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

type errOnNodeGetClient struct {
	client.Client
	name types.NamespacedName
	err  error
}

func (c errOnNodeGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Node); ok && key == c.name {
		return c.err
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

type switchingMCMGetClient struct {
	client.Client
	name   types.NamespacedName
	status *mcmv1alpha1.MachineStatus
}

func (c *switchingMCMGetClient) SetStatus(status mcmv1alpha1.MachineStatus) {
	c.status = &status
}

func (c *switchingMCMGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := c.Client.Get(ctx, key, obj, opts...)
	if err != nil {
		return err
	}

	machineObj, ok := obj.(*mcmv1alpha1.Machine)
	if !ok || key != c.name || c.status == nil {
		return nil
	}

	machineObj.Status = *c.status
	return nil
}

type conflictOnInstanceStatusPatchClient struct {
	client.Client
	err error
}

func (c conflictOnInstanceStatusPatchClient) Status() client.SubResourceWriter {
	return conflictOnStatusWriter{SubResourceWriter: c.Client.Status(), err: c.err}
}

type conflictOnStatusWriter struct {
	client.SubResourceWriter
	err error
}

func (w conflictOnStatusWriter) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.SubResourcePatchOption,
) error {
	if _, ok := obj.(*deckhousev1alpha2.Instance); ok {
		return w.err
	}

	return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
}
