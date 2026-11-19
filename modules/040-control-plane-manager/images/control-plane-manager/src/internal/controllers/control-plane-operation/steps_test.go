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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"

	cp "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpn/cpnplanner"
)

func TestCertObserveStep_RetryReleasesApprovalSlot(t *testing.T) {
	signatureMode(t, "false")
	pkiDir, configDir := newObservationFixture(t)
	require.NoError(t, os.Remove(filepath.Join(configDir, "super-admin.conf")))
	steps := []cp.StepName{cp.StepSyncManifests, cp.StepCertObserve}
	failed := testOperation(cp.OperationComponentKubeAPIServer, steps, true)
	state := cp.NewOperationState(failed)
	state.MarkStepCompleted(cp.StepSyncManifests)
	state.MarkStepFailed(cp.StepCertObserve, "missing super-admin.conf")
	state.MarkOperationFailed("missing super-admin.conf")
	var calls []execCall
	registry := map[cp.StepName]Step{
		cp.StepSyncManifests: newMockError(&calls, cp.StepSyncManifests, errors.New("completed write repeated")),
		cp.StepCertObserve:   &certObserveStep{pkiDir: pkiDir},
	}
	r := newTestReconciler(registry, failed, testCPMSecret(), testPKISecret())
	r.node.KubeconfigDir = configDir
	queued := testOperation(cp.OperationComponentKubeAPIServer, steps, false)
	queued.Name = "queued"
	queued.Spec.NodeName = "master-2"
	queued.Labels[constants.ControlPlaneNodeNameLabelKey] = "master-2"
	a := approver.NewApprover(approver.StaticPodPipeline)
	nodes := approver.Nodes{Masters: 3}
	require.Empty(t, a.SelectApprovable([]cp.ControlPlaneOperation{*failed, *queued}, nodes))

	_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Name: failed.Name}})

	require.NoError(t, err)
	completed := &cp.ControlPlaneOperation{}
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: failed.Name}, completed))
	require.True(t, completed.IsCompleted())
	require.True(t, completed.IsTerminal())
	require.True(t, completed.IsStepCompleted(cp.StepCertObserve))
	require.Empty(t, calls)
	require.NotNil(t, completed.Status.ObservedState)
	expiry := completed.Status.ObservedState.CertificatesExpirationTime["super-admin.conf"]
	require.Equal(t, "1970-01-01T00:00:00Z", expiry.UTC().Format(time.RFC3339))
	admitted := a.SelectApprovable([]cp.ControlPlaneOperation{*completed, *queued}, nodes)
	require.Len(t, admitted, 1)
	require.Equal(t, queued.Name, admitted[0].Name)
}

func TestCertObserveStep_MissingExternalCertificateSchedulesRecovery(t *testing.T) {
	signatureMode(t, "false")
	pkiDir, configDir := newObservationFixture(t)
	configPath := filepath.Join(configDir, "super-admin.conf")
	config, err := clientcmd.LoadFromFile(configPath)
	require.NoError(t, err)
	auth := config.AuthInfos[config.Contexts[config.CurrentContext].AuthInfo]
	auth.ClientCertificateData = nil
	auth.ClientCertificate = "missing-client.crt"
	require.NoError(t, clientcmd.WriteToFile(*config, configPath))
	observation := testOperation(cp.OperationComponentKubeAPIServer, []cp.StepName{cp.StepCertObserve}, true)
	r := newTestReconciler(map[cp.StepName]Step{cp.StepCertObserve: &certObserveStep{pkiDir: pkiDir}}, observation)
	r.node.KubeconfigDir = configDir

	// Observation completes without secrets or recovering the missing credential.
	_, err = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Name: observation.Name}})
	require.NoError(t, err)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: observation.Name}, observation))
	require.True(t, observation.IsCompleted())
	require.Empty(t, observation.Status.ChangedFiles)
	_, err = kubeconfig.GetClientCertificateExpiration(configPath)
	require.Error(t, err)

	// The real payload drives the existing, approval-gated renewal pipeline.
	cpn := nodeForObservedOperation(observation)
	plan := cpnplanner.ComputePlan(cpn, []cp.ControlPlaneOperation{*observation}, cpnplanner.StaticPodOperationBuilder{})
	require.NotNil(t, plan.Status)
	expiry := plan.Status.Components.KubeAPIServer.CertificatesExpirationTime["super-admin.conf"]
	require.Equal(t, "1970-01-01T00:00:00Z", expiry.UTC().Format(time.RFC3339))
	require.Len(t, plan.Create, 1)
	require.False(t, plan.Create[0].Op.Spec.Approved)
	require.Equal(t, []cp.StepName{
		cp.StepBackup, cp.StepSyncCA, cp.StepRenewPKICerts, cp.StepRenewKubeconfigs,
		cp.StepSyncManifests, cp.StepWaitPodReady, cp.StepCertObserve,
	}, plan.Create[0].Op.Spec.Steps)

	// Execute only the real kubeconfig generator; cluster rollout steps are not run.
	changed, err := renewKubeconfigsForComponent(cp.OperationComponentKubeAPIServer,
		map[string][]byte{constants.SecretKeyEncryptionAlgorithm: []byte("ECDSA-P256")}, pkiDir, configDir, "192.0.2.10")
	require.NoError(t, err)
	require.True(t, changed)
	config, err = clientcmd.LoadFromFile(configPath)
	require.NoError(t, err)
	auth = config.AuthInfos[config.Contexts[config.CurrentContext].AuthInfo]
	require.NotEmpty(t, auth.ClientCertificateData)
	require.NotEmpty(t, auth.ClientKeyData)
	_, err = os.Stat(filepath.Join(configDir, "missing-client.crt"))
	require.ErrorIs(t, err, os.ErrNotExist)
	restored, err := kubeconfig.GetClientCertificateExpiration(configPath)
	require.NoError(t, err)
	require.True(t, restored.NotAfter.After(time.Now().Add(30*24*time.Hour)))

	// A fresh completed observation replaces epoch with the generated certificate's real date.
	after := testOperation(cp.OperationComponentKubeAPIServer, []cp.StepName{cp.StepCertObserve}, true)
	after.Name = "after-renewal"
	require.NoError(t, r.client.Create(context.Background(), after))
	_, err = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Name: after.Name}})
	require.NoError(t, err)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: after.Name}, after))
	require.True(t, after.IsCompleted())
	cpn.Status = *plan.Status
	cpn.Status = cpnplanner.ComputeStatusReport(cpn, []cp.ControlPlaneOperation{*after})
	expiry = cpn.Status.Components.KubeAPIServer.CertificatesExpirationTime["super-admin.conf"]
	require.True(t, restored.NotAfter.Equal(expiry.Time))
	require.Empty(t, cpnplanner.ComputePlan(cpn, []cp.ControlPlaneOperation{*after}, cpnplanner.StaticPodOperationBuilder{}).Create)
}

func TestCertObserveStep_MissingCertificateDoesNotHideSignatureError(t *testing.T) {
	signatureMode(t, "true")
	pkiDir, configDir := newObservationFixture(t)
	require.NoError(t, os.Remove(filepath.Join(configDir, "super-admin.conf")))
	require.NoError(t, os.WriteFile(filepath.Join(pkiDir, "signature-private.jwk"), []byte("invalid JSON"), 0600))
	op := testOperation(cp.OperationComponentKubeAPIServer, []cp.StepName{cp.StepCertObserve}, true)
	r := newTestReconciler(map[cp.StepName]Step{cp.StepCertObserve: &certObserveStep{pkiDir: pkiDir}}, op)
	r.node.KubeconfigDir = configDir

	_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Name: op.Name}})

	require.ErrorContains(t, err, "signature key: parse private jwk")
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: op.Name}, op))
	require.True(t, op.IsFailed())
	require.False(t, op.IsTerminal())
	require.NotNil(t, op.Status.ObservedState)
	expiry := op.Status.ObservedState.CertificatesExpirationTime["super-admin.conf"]
	require.Equal(t, "1970-01-01T00:00:00Z", expiry.UTC().Format(time.RFC3339))
	require.NotContains(t, op.Status.ObservedState.CertificatesExpirationTime, constants.SignatureExpirationKey)
}

func signatureMode(t *testing.T, enabled string) {
	t.Helper()
	previous := constants.SignatureBuildEnabled
	constants.SignatureBuildEnabled = enabled
	t.Cleanup(func() { constants.SignatureBuildEnabled = previous })
}

func nodeForObservedOperation(op *cp.ControlPlaneOperation) *cp.ControlPlaneNode {
	checksums := cp.Checksums{Config: op.Spec.DesiredConfigChecksum, PKI: op.Spec.DesiredPKIChecksum, CA: op.Spec.DesiredCAChecksum}
	cpn := &cp.ControlPlaneNode{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	cpn.Spec.CAChecksum = checksums.CA
	cpn.Spec.Components.KubeAPIServer.Checksums = cp.Checksums{Config: checksums.Config, PKI: checksums.PKI}
	cpn.Status.CAChecksum = checksums.CA
	cpn.Status.Components.KubeAPIServer.Checksums = checksums
	return cpn
}
