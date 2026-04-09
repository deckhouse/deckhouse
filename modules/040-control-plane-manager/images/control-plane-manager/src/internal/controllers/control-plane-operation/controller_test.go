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
	name      controlplanev1alpha1.CommandName
	component controlplanev1alpha1.OperationComponent
}

type mockCommand struct {
	name   controlplanev1alpha1.CommandName
	result reconcile.Result
	err    error
	calls  *[]execCall
}

func (m *mockCommand) Execute(_ context.Context, env *CommandEnv, _ *log.Logger) (reconcile.Result, error) {
	*m.calls = append(*m.calls, execCall{name: m.name, component: env.State.Raw().Spec.Component})
	return m.result, m.err
}

func newMockOK(calls *[]execCall, name controlplanev1alpha1.CommandName) Command {
	return &mockCommand{name: name, calls: calls}
}

func newMockError(calls *[]execCall, name controlplanev1alpha1.CommandName, err error) Command {
	return &mockCommand{name: name, err: err, calls: calls}
}

func newMockRequeue(calls *[]execCall, name controlplanev1alpha1.CommandName, after time.Duration) Command {
	return &mockCommand{name: name, result: reconcile.Result{RequeueAfter: after}, calls: calls}
}

// helpers

const (
	testNodeName      = "master-1"
	testConfigVersion = "100.200"
)

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

func testOperation(component controlplanev1alpha1.OperationComponent, commands []controlplanev1alpha1.CommandName, approved bool) *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-op",
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  testNodeName,
				constants.ControlPlaneComponentLabelKey: string(component),
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			ConfigVersion:         testConfigVersion,
			NodeName:              testNodeName,
			Component:             component,
			Commands:              commands,
			DesiredConfigChecksum: "config-hash",
			Approved:              approved,
		},
	}
}

// buildTestCase builds a mock registry and a matching approved operation from the given mocks.
// Command names are derived from the mocks — no need to repeat them in both places.
func buildTestCase(component controlplanev1alpha1.OperationComponent, mocks ...Command) (map[controlplanev1alpha1.CommandName]Command, *controlplanev1alpha1.ControlPlaneOperation) {
	cmds := make(map[controlplanev1alpha1.CommandName]Command, len(mocks))
	names := make([]controlplanev1alpha1.CommandName, 0, len(mocks))
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

func (s *ControllerTestSuite) newReconciler(cmds map[controlplanev1alpha1.CommandName]Command, objs ...client.Object) *Reconciler {
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
		Build()
	if cmds == nil {
		cmds = defaultCommands()
	}
	return &Reconciler{
		client:   c,
		log:      log.NewNop(),
		node:     NodeIdentity{Name: testNodeName},
		commands: cmds,
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
	registry := defaultCommands()

	s.Run("valid commands resolved", func() {
		err := resolveCommands(registry, []controlplanev1alpha1.CommandName{
			controlplanev1alpha1.CommandSyncCA,
			controlplanev1alpha1.CommandSyncManifests,
		})
		require.NoError(s.T(), err)
	})

	s.Run("unknown command returns error", func() {
		err := resolveCommands(registry, []controlplanev1alpha1.CommandName{
			controlplanev1alpha1.CommandSyncCA,
			"BogusCommand",
		})
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "unknown command")
	})

	s.Run("empty list returns no error", func() {
		err := resolveCommands(registry, []controlplanev1alpha1.CommandName{})
		require.NoError(s.T(), err)
	})
}

func (s *ControllerTestSuite) TestReconcileNotApproved() {
	s.Run("not approved operation is skipped", func() {
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler,
			[]controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandSyncManifests}, false)
		r := s.newReconciler(nil, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		got := s.getOp(r, "test-op")
		require.Empty(s.T(), got.Status.Conditions, "no conditions should be set")
	})
}

func (s *ControllerTestSuite) TestReconcileAlreadyCompleted() {
	s.Run("completed operation is skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: constants.ConditionReady, Status: metav1.ConditionTrue, Reason: constants.ReasonOperationSucceeded, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Empty(s.T(), calls, "no commands should execute")
	})
}

func (s *ControllerTestSuite) TestReconcileAlreadyFailed() {
	s.Run("failed operation is skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: constants.ConditionFailed, Status: metav1.ConditionTrue, Reason: constants.ReasonCommandFailed, Message: "boom", LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Empty(s.T(), calls, "no commands should execute")
	})
}

func (s *ControllerTestSuite) TestReconcileConfigVersionMismatch() {
	s.Run("configVersion mismatch cancels operation", func() {
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler,
			[]controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandSyncManifests}, true)
		op.Spec.ConfigVersion = "old.version"
		r := s.newReconciler(nil, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), constants.ReasonCancelled, readyCond.Reason)
	})
}

func (s *ControllerTestSuite) TestReconcileCertObserveNoSecrets() {
	s.Run("CertObserver component runs CertObserve command without reading secrets", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentCertObserver,
			newMockOK(&calls, controlplanev1alpha1.CommandCertObserve),
		)
		// No secrets in cluster — CertObserve should not need them
		r := s.newReconciler(cmds, op)

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)
		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.CommandCertObserve, calls[0].name)
	})
}

func (s *ControllerTestSuite) TestPipelineAllCommandsExecuteInOrder() {
	s.Run("all commands execute sequentially", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.CommandWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 3)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncManifests, calls[1].name)
		require.Equal(s.T(), controlplanev1alpha1.CommandWaitPodReady, calls[2].name)

		// Verify Ready condition
		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)
		require.Equal(s.T(), constants.ReasonOperationSucceeded, readyCond.Reason)

		// Verify Failed=False
		failedCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionFailed)
		require.NotNil(s.T(), failedCond)
		require.Equal(s.T(), metav1.ConditionFalse, failedCond.Status)
	})
}

func (s *ControllerTestSuite) TestPipelineSkipsCompletedCommands() {
	s.Run("completed commands are skipped", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.CommandWaitPodReady),
		)
		// Mark first command as already completed
		op.Status.Conditions = []metav1.Condition{
			{Type: string(controlplanev1alpha1.CommandSyncCA), Status: metav1.ConditionTrue, Reason: constants.ReasonCommandCompleted, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		// SyncCA was skipped
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncManifests, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.CommandWaitPodReady, calls[1].name)
	})
}

func (s *ControllerTestSuite) TestPipelineSkipsMultipleCompletedCommands() {
	s.Run("first two completed, only third executes", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
			newMockOK(&calls, controlplanev1alpha1.CommandWaitPodReady),
		)
		op.Status.Conditions = []metav1.Condition{
			{Type: string(controlplanev1alpha1.CommandSyncCA), Status: metav1.ConditionTrue, Reason: constants.ReasonCommandCompleted, LastTransitionTime: metav1.Now()},
			{Type: string(controlplanev1alpha1.CommandSyncManifests), Status: metav1.ConditionTrue, Reason: constants.ReasonCommandCompleted, LastTransitionTime: metav1.Now()},
		}
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.CommandWaitPodReady, calls[0].name)
	})
}

func (s *ControllerTestSuite) TestPipelineErrorStopsPipeline() {
	s.Run("command error stops pipeline and propagates error", func() {
		var calls []execCall
		cmdErr := fmt.Errorf("disk full")
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockError(&calls, controlplanev1alpha1.CommandSyncManifests, cmdErr),
			newMockOK(&calls, controlplanev1alpha1.CommandWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "disk full")

		// cmd1 executed, cmd2 executed (and failed), cmd3 NOT executed
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncManifests, calls[1].name)

		// Verify failed command condition
		got := s.getOp(r, "test-op")
		cmdCond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncManifests))
		require.NotNil(s.T(), cmdCond)
		require.Equal(s.T(), metav1.ConditionFalse, cmdCond.Status)
		require.Equal(s.T(), constants.ReasonCommandFailed, cmdCond.Reason)
		require.Contains(s.T(), cmdCond.Message, "disk full")

		// Ready should NOT be true
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		if readyCond != nil {
			require.NotEqual(s.T(), metav1.ConditionTrue, readyCond.Status)
		}
	})
}

func (s *ControllerTestSuite) TestPipelineRequeueStopsPipeline() {
	s.Run("command requeue stops pipeline without marking ready", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockRequeue(&calls, controlplanev1alpha1.CommandSyncManifests, 5*time.Second),
			newMockOK(&calls, controlplanev1alpha1.CommandWaitPodReady),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), 5*time.Second, result.RequeueAfter)

		// cmd1 executed, cmd2 executed (returned requeue), cmd3 NOT executed
		require.Len(s.T(), calls, 2)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncCA, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncManifests, calls[1].name)

		// Ready should NOT be true
		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		if readyCond != nil {
			require.NotEqual(s.T(), metav1.ConditionTrue, readyCond.Status)
		}
	})
}

func (s *ControllerTestSuite) TestPipelineSingleCommand() {
	s.Run("single command pipeline completes successfully", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentHotReload,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncHotReload),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		result, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)
		require.Equal(s.T(), reconcile.Result{}, result)

		require.Len(s.T(), calls, 1)
		require.Equal(s.T(), controlplanev1alpha1.CommandSyncHotReload, calls[0].name)
		require.Equal(s.T(), controlplanev1alpha1.OperationComponentHotReload, calls[0].component)

		got := s.getOp(r, "test-op")
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)
	})
}

func (s *ControllerTestSuite) TestConditionsAfterSuccessfulPipeline() {
	s.Run("successful pipeline sets per-command conditions and Ready", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeAPIServer,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockOK(&calls, controlplanev1alpha1.CommandSyncManifests),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, err := r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})
		require.NoError(s.T(), err)

		got := s.getOp(r, "test-op")

		// Each command condition = True
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)
		require.Equal(s.T(), constants.ReasonCommandCompleted, syncCACond.Reason)

		syncManifestsCond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncManifests))
		require.NotNil(s.T(), syncManifestsCond)
		require.Equal(s.T(), metav1.ConditionTrue, syncManifestsCond.Status)
		require.Equal(s.T(), constants.ReasonCommandCompleted, syncManifestsCond.Reason)

		// Ready = True
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionTrue, readyCond.Status)

		// Failed = False
		failedCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionFailed)
		require.NotNil(s.T(), failedCond)
		require.Equal(s.T(), metav1.ConditionFalse, failedCond.Status)
	})
}

func (s *ControllerTestSuite) TestConditionsAfterError() {
	s.Run("error sets first command completed, second command failed", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeAPIServer,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockError(&calls, controlplanev1alpha1.CommandSyncManifests, fmt.Errorf("write failed")),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, _ = r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})

		got := s.getOp(r, "test-op")

		// First command completed
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)

		// Second command failed
		syncManifestsCond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncManifests))
		require.NotNil(s.T(), syncManifestsCond)
		require.Equal(s.T(), metav1.ConditionFalse, syncManifestsCond.Status)
		require.Equal(s.T(), constants.ReasonCommandFailed, syncManifestsCond.Reason)
		require.Contains(s.T(), syncManifestsCond.Message, "write failed")
	})
}

func (s *ControllerTestSuite) TestConditionsAfterRequeue() {
	s.Run("requeue sets command InProgress, Ready stays false", func() {
		var calls []execCall
		cmds, op := buildTestCase(controlplanev1alpha1.OperationComponentKubeScheduler,
			newMockOK(&calls, controlplanev1alpha1.CommandSyncCA),
			newMockRequeue(&calls, controlplanev1alpha1.CommandWaitPodReady, 5*time.Second),
		)
		r := s.newReconciler(cmds, op, testCPMSecret(), testPKISecret())

		_, _ = r.Reconcile(s.ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-op"}})

		got := s.getOp(r, "test-op")

		// First command completed
		syncCACond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandSyncCA))
		require.NotNil(s.T(), syncCACond)
		require.Equal(s.T(), metav1.ConditionTrue, syncCACond.Status)

		// Second command still in progress
		waitCond := meta.FindStatusCondition(got.Status.Conditions, string(controlplanev1alpha1.CommandWaitPodReady))
		require.NotNil(s.T(), waitCond)
		require.Equal(s.T(), metav1.ConditionFalse, waitCond.Status)
		require.Equal(s.T(), constants.ReasonCommandInProgress, waitCond.Reason)

		// Ready still false
		readyCond := meta.FindStatusCondition(got.Status.Conditions, constants.ConditionReady)
		require.NotNil(s.T(), readyCond)
		require.Equal(s.T(), metav1.ConditionFalse, readyCond.Status)
		require.Equal(s.T(), constants.ReasonWaitingForPod, readyCond.Reason)
	})
}
