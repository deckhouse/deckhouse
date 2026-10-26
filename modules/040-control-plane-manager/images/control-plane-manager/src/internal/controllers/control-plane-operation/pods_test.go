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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func TestWaitForPod_CertRenewal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	op := &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "etcd-renew"},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:           testNodeName,
			Component:          controlplanev1alpha1.OperationComponentEtcd,
			DesiredPKIChecksum: "pki",
			Steps: []controlplanev1alpha1.StepName{
				controlplanev1alpha1.StepRenewPKICerts,
				controlplanev1alpha1.StepJoinEtcdCluster,
				controlplanev1alpha1.StepWaitPodReady,
			},
		},
	}
	op.Status.Conditions = []metav1.Condition{{
		Type:    controlplanev1alpha1.StepConditionType(controlplanev1alpha1.StepRenewPKICerts),
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1alpha1.CPOReasonStepCompleted,
		Message: controlplanev1alpha1.CPOStepResultRenewed,
	}}
	// The old pod: Ready with the unchanged PKI checksum, but not restarted with the renewed certificates.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", controlplanev1alpha1.OperationComponentEtcd.PodComponentName(), testNodeName),
			Namespace:   constants.KubeSystemNamespace,
			Annotations: map[string]string{constants.PKIChecksumAnnotationKey: "pki"},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	r := newTestReconciler(nil, pod)
	state := controlplanev1alpha1.NewOperationState(op)

	res, err := r.waitForPod(ctx, state, log.NewNop())
	require.NoError(t, err)
	require.Equal(t, OutcomePending, res.Outcome)

	pod.Annotations[constants.CertRenewalIDAnnotationKey] = op.Name
	require.NoError(t, r.client.Update(ctx, pod))
	res, err = r.waitForPod(ctx, state, log.NewNop())
	require.NoError(t, err)
	require.Equal(t, OutcomeCompleted, res.Outcome)
}
