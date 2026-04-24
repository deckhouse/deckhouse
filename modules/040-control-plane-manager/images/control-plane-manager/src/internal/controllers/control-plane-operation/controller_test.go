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

package controlplaneoperation

import (
	"context"
	"fmt"
	"testing"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

// mocks

type execCall struct {
	name      controlplanev1alpha1.StepName
	component controlplanev1alpha1.OperationComponent
}

type mockCommand struct {
	name   controlplanev1alpha1.StepName
	result StepResult
	err    error
	calls  *[]execCall
}

func (m *mockCommand) Execute(_ context.Context, env *StepEnv, _ *log.Logger) (StepResult, error) {
	*m.calls = append(*m.calls, execCall{name: m.name, component: env.State.Raw().Spec.Component})
	return m.result, m.err
}

func newMockOK(calls *[]execCall, name controlplanev1alpha1.StepName) Step {
	return &mockCommand{name: name, result: StepResult{Outcome: OutcomeCompleted}, calls: calls}
}

func newMockError(calls *[]execCall, name controlplanev1alpha1.StepName, err error) Step {
	return &mockCommand{name: name, err: err, calls: calls}
}

func newMockRequeue(calls *[]execCall, name controlplanev1alpha1.StepName, after time.Duration) Step {
	return &mockCommand{name: name, result: StepResult{Outcome: OutcomePending, RequeueAfter: after}, calls: calls}
}

func newMockCompleteWithMessage(calls *[]execCall, name controlplanev1alpha1.StepName, message string) Step {
	return &mockCommand{name: name, result: StepResult{Outcome: OutcomeCompleted, Message: message}, calls: calls}
}

// helpers

const testNodeName = "master-1"

func testCPMSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            constants.ControlPlaneManagerConfigSecretName,
			Namespace:       constants.KubeSystemNamespace,
			ResourceVersion: "100",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
}

func testPKISecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            constants.PkiSecretName,
			Namespace:       constants.KubeSystemNamespace,
			ResourceVersion: "200",
		},
		Data: map[string][]byte{"ca.crt": []byte("ca-data")},
	}
}

func testOperation(component controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName, approved bool) *controlplanev1alpha1.ControlPlaneOperation {
	desiredConfigChecksum, desiredPKIChecksum, desiredCAChecksum := desiredChecksumsForComponent(component)

	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-op",
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  testNodeName,
				constants.ControlPlaneComponentLabelKey: component.LabelValue(),
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:              testNodeName,
			Component:             component,
			Steps:                 steps,
			DesiredConfigChecksum: desiredConfigChecksum,
			DesiredPKIChecksum:    desiredPKIChecksum,
			DesiredCAChecksum:     desiredCAChecksum,
			Approved:              approved,
		},
	}
}

func desiredChecksumsForComponent(component controlplanev1alpha1.OperationComponent) (string, string, string) {
	cpmSecretData := testCPMSecret().Data
	pkiSecretData := testPKISecret().Data

	switch component {
	case controlplanev1alpha1.OperationComponentCertObserver:
		return "", "", ""
	default:
		podName := component.PodComponentName()
		configChecksum, err := checksum.ComponentChecksum(cpmSecretData, podName)
		if err != nil {
			panic(fmt.Sprintf("failed to compute config checksum in test helper: %v", err))
		}
		pkiChecksum, err := checksum.ComponentPKIChecksum(cpmSecretData, podName)
		if err != nil {
			panic(fmt.Sprintf("failed to compute pki checksum in test helper: %v", err))
		}
		caChecksum, err := checksum.PKIChecksum(pkiSecretData)
		if err != nil {
			panic(fmt.Sprintf("failed to compute ca checksum in test helper: %v", err))
		}
		return configChecksum, pkiChecksum, caChecksum
	}
}

// buildTestCase builds a mock registry and a matching approved operation from the given mocks.
// Step names are derived from the mocks — no need to repeat them in both places.
func buildTestCase(component controlplanev1alpha1.OperationComponent, mocks ...Step) (map[controlplanev1alpha1.StepName]Step, *controlplanev1alpha1.ControlPlaneOperation) {
	cmds := make(map[controlplanev1alpha1.StepName]Step, len(mocks))
	names := make([]controlplanev1alpha1.StepName, 0, len(mocks))
	for _, m := range mocks {
		name := m.(*mockCommand).name
		cmds[name] = m
		names = append(names, name)
	}
	return cmds, testOperation(component, names, true)
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ControllerTestSuite) SetupSuite() {
	s.ctx = context.Background()
}

func (s *ControllerTestSuite) newReconciler(cmds map[controlplanev1alpha1.StepName]Step, objs ...client.Object) *Reconciler {
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
		Build()
	if cmds == nil {
		cmds = defaultSteps()
	}
	return &Reconciler{
		client: c,
		log:    log.NewNop(),
		node:   NodeIdentity{Name: testNodeName},
		steps:  cmds,
	}
}

func (s *ControllerTestSuite) getOp(r *Reconciler, name string) *controlplanev1alpha1.ControlPlaneOperation {
	op := &controlplanev1alpha1.ControlPlaneOperation{}
	err := r.client.Get(s.ctx, client.ObjectKey{Name: name}, op)
	require.NoError(s.T(), err)
	return op
}

// Tests

func (s *ControllerTestSuite) TestResolveCommands() {
	registry := defaultSteps()

	s.Run("valid steps resolved", func() {
		err := resolveSteps(registry, []controlplanev1alpha1.StepName{
			controlplanev1alpha1.StepSyncCA,
			controlplanev1alpha1.StepSyncManifests,
		})
		require.NoError(s.T(), err)
	})

	s.Run("unknown step returns error", func() {
		err := resolveSteps(registry, []controlplanev1alpha1.StepName{
			controlplanev1alpha1.StepSyncCA,
			"BogusCommand",
		})
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "unknown step")
	})

	s.Run("empty list returns no error", func() {
		err := resolveSteps(registry, []controlplanev1alpha1.StepName{})
		require.NoError(s.T(), err)
	})
}

func (s *ControllerTestSuite) TestReconcileNotApproved() {
	s.Run("not approved operation is skipped", func() {
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler,
			[]controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests}, false)
		r := s.newReconciler(nil, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		got := s.getOp(r, "test-op")
		require.Len(s.T(), got.Status.Conditions, 2)

		completedCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), completedCond)
		require.Equal(s.T(), metav1.ConditionUnknown, completedCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationUnknown, completedCond.Reason)

		commandCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionManifestsSynced)
		require.NotNil(s.T(), commandCond)
		require.Equal(s.T(), metav1.ConditionFalse, commandCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepUnknown, commandCond.Reason)
	})
}

func (s *ControllerTestSuite) TestReconcileInitialConditionsPreserveExisting() {
	s.Run("initialization keeps existing conditions unchanged", func() {
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler,
			[]controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests}, false)
		op.Status.Conditions = []metav1.Condition{
			{
				Type:   controlplanev1alpha1.CPOConditionCompleted,
				Status: metav1.ConditionFalse,
				Reason: controlplanev1alpha1.CPOReasonOperationInProgress,
			},
			{
				Type:   controlplanev1alpha1.CPOConditionManifestsSynced,
				Status: metav1.ConditionTrue,
				Reason: controlplanev1alpha1.CPOReasonStepCompleted,
			},
		}

		r := s.newReconciler(nil, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		got := s.getOp(r, "test-op")
		require.Len(s.T(), got.Status.Conditions, 2)

		completedCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), completedCond)
		require.Equal(s.T(), metav1.ConditionFalse, completedCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationInProgress, completedCond.Reason)

		commandCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionManifestsSynced)
		require.NotNil(s.T(), commandCond)
		require.Equal(s.T(), metav1.ConditionTrue, commandCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepCompleted, commandCond.Reason)
	})
}

func (s *ControllerTestSuite) TestReconcileAlreadyCompleted() {
	s.Run("completed operation is skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: controlplanev1alpha1.CPOConditionCompleted, Status: metav1.ConditionTrue, Reason: controlplanev1alpha1.CPOReasonOperationCompleted, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Empty(s.T(), calls, "no steps should execute")
	})
}

func (s *ControllerTestSuite) TestReconcileAlreadyFailed() {
	s.Run("failed operation is skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: controlplanev1alpha1.CPOConditionCompleted, Status: metav1.ConditionFalse, Reason: controlplanev1alpha1.CPOReasonOperationFailed, Message: "boom", LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Empty(s.T(), calls, "no steps should execute")
	})
}

func (s *ControllerTestSuite) TestReconcileChecksumMismatch() {
	s.Run("checksum mismatch abandons operation", func() {
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler,
			[]controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests}, true)
		op.Spec.DesiredConfigChecksum = "stale-checksum"
		r := s.newReconciler(nil, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationAbandoned, readyCond.Reason)
	})
}

func (s *ControllerTestSuite) TestReconcileObserveOnlyNoSecrets() {
	s.Run("observe-only operation runs CertObserve without reading secrets", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepCertObserve),
		)
		op.Spec.DesiredConfigChecksum = ""
		op.Spec.DesiredPKIChecksum = ""
		op.Spec.DesiredCAChecksum = ""
		// No secrets in cluster — CertObserve should not need them
		r := s.newReconciler(cmds, op)

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.StepCertObserve, calls[0].name)
	})
}

func (s *ControllerTestSuite) TestPipelineAllCommandsExecuteInOrder() {
	s.Run("all steps execute sequentially", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 3)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncManifests, calls[1].name)
		require.Equal(s.T(), controlplanev1alpha1.StepWaitPodReady, calls[2].name)

		// Verify Ready condition
		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationCompleted, readyCond.Reason)

	})
}

func (s *ControllerTestSuite) TestPipelineSkipsCompletedCommands() {
	s.Run("completed steps are skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		// Mark first command as already completed
		op.Status.Conditions = []metav1.Condition{
			{Type: controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA), Status: metav1.ConditionTrue, Reason: controlplanev1alpha1.CPOReasonStepCompleted, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		// SyncCA was skipped
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncManifests, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.StepWaitPodReady, calls[1].name)
	})
}

func (s *ControllerTestSuite) TestPipelineSkipsMultipleCompletedCommands() {
	s.Run("first two completed, only third executes", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA), Status: metav1.ConditionTrue, Reason: controlplanev1alpha1.CPOReasonStepCompleted, LastTransitionTime: metav1.Now()},
			{Type: controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncManifests), Status: metav1.ConditionTrue, Reason: controlplanev1alpha1.CPOReasonStepCompleted, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.StepWaitPodReady, calls[0].name)
	})
}

func (s *ControllerTestSuite) TestPipelineErrorStopsPipeline() {
	s.Run("command error stops pipeline and propagates error", func() {
		var calls []execCall
		cmdErr := fmt.Errorf("disk full")
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockError(&calls, controlplanev1alpha1.StepSyncManifests, cmdErr),
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "disk full")

		// cmd1 executed, cmd2 executed (and failed), cmd3 NOT executed
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncManifests, calls[1].name)

		// Verify failed command condition
		got := s.getOp(r, "test-op")
		cmdCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncManifests))
		require.NotNil(s.T(), cmdCond)
		require.Equal(s.T(), metav1.ConditionFalse, cmdCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepFailed, cmdCond.Reason)
		require.Contains(s.T(), cmdCond.Message, "disk full")

		// Ready should reflect terminal operation failure
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationFailed, readyCond.Reason)
	})
}

func (s *ControllerTestSuite) TestPipelineRequeueStopsPipeline() {
	s.Run("command requeue stops pipeline without marking ready", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockRequeue(&calls, controlplanev1alpha1.StepSyncManifests, 5*time.Second),
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), 5*time.Second, result.RequeueAfter)

		// cmd1 executed, cmd2 executed (returned requeue), cmd3 NOT executed
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.StepSyncManifests, calls[1].name)

		// Ready should stay in progress
		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationInProgress, readyCond.Reason)
	})
}

func (s *ControllerTestSuite) TestPipelineSingleCommand() {
	s.Run("single command pipeline completes successfully", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.StepWaitPodReady, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.OperationComponentKubeScheduler, calls[0].component)

		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)
	})
}

func (s *ControllerTestSuite) TestConditionsAfterSuccessfulPipeline() {
	s.Run("successful pipeline sets per-command conditions and Ready", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeAPIServer,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockOK(&calls, controlplanev1alpha1.StepSyncManifests),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)

		got := s.getOp(r, "test-op")

		// Each command condition = True
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepCompleted, syncCACond.Reason)

		syncManifestsCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncManifests))
		require.NotNil(s.T(), syncManifestsCond)
		require.Equal(s.T(), metav1.ConditionTrue, syncManifestsCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepCompleted, syncManifestsCond.Reason)

		// Ready = True
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)

	})
}

func (s *ControllerTestSuite) TestConditionsAfterError() {
	s.Run("error sets first command completed, second step failed", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeAPIServer,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockError(&calls, controlplanev1alpha1.StepSyncManifests, fmt.Errorf("write failed")),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, _ = r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})

		got := s.getOp(r, "test-op")

		// First command completed
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)

		// Second step failed
		syncManifestsCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncManifests))
		require.NotNil(s.T(), syncManifestsCond)
		require.Equal(s.T(), metav1.ConditionFalse, syncManifestsCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepFailed, syncManifestsCond.Reason)
		require.Contains(s.T(), syncManifestsCond.Message, "write failed")

		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationFailed, readyCond.Reason)
	})
}

func (s *ControllerTestSuite) TestConditionsAfterRequeue() {
	s.Run("requeue sets command InProgress, Ready stays in progress", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.StepSyncCA),
			newMockRequeue(&calls, controlplanev1alpha1.StepWaitPodReady, 5*time.Second),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, _ = r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})

		got := s.getOp(r, "test-op")

		// First command completed
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)

		// Second command still in progress
		waitCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepWaitPodReady))
		require.NotNil(s.T(), waitCond)
		require.Equal(s.T(), metav1.ConditionFalse, waitCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepInProgress, waitCond.Reason)

		// Ready still in progress
		readyCond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationInProgress, readyCond.Reason)
		readyCondFirstTransition := readyCond.LastTransitionTime

		// Requeue again: Completed condition should stay OperationInProgress with unchanged transition time.
		_, _ = r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		got = s.getOp(r, "test-op")
		readyCond = meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonOperationInProgress, readyCond.Reason)
		require.True(s.T(), readyCond.LastTransitionTime.Equal(&readyCondFirstTransition),
			"Completed transition time must not change on requeue when status/reason are unchanged")
	})
}

func (s *ControllerTestSuite) TestPipelineKeepsCommandCompletedMessage() {
	s.Run("pipeline does not overwrite command completed message", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockCompleteWithMessage(&calls, controlplanev1alpha1.StepSyncCA, controlplanev1alpha1.CPOStepResultRenewed),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)

		got := s.getOp(r, "test-op")
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)
		require.Equal(s.T(), controlplanev1alpha1.CPOReasonStepCompleted, syncCACond.Reason)
		require.Equal(s.T(), controlplanev1alpha1.CPOStepResultRenewed, syncCACond.Message)
	})
}
