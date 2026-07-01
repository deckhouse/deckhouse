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

package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const statefulSetRecreateRequeue = 5 * time.Second

func (e *StepExecutor) syncManifests(ctx context.Context) operations.StepResult {
	const step = controlplanev1alpha1.StepSyncManifests

	target, err := e.buildTargetStatefulSet(ctx)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}

	current := &appsv1.StatefulSet{}
	err = e.client.Get(ctx, client.ObjectKeyFromObject(target), current)
	if apierrors.IsNotFound(err) {
		if err := e.client.Create(ctx, target); err != nil {
			return operations.StepHasFailed(step, fmt.Errorf("create statefulset: %w", err))
		}
		return operations.StepIsCompleted(step, "statefulset created")
	}
	if err != nil {
		return operations.StepHasFailed(step, fmt.Errorf("get statefulset: %w", err))
	}

	if !current.DeletionTimestamp.IsZero() {
		return operations.StepIsProgressing(
			step,
			fmt.Sprintf("waiting for statefulset %s to terminate before recreate", target.Name),
			statefulSetRecreateRequeue)
	}

	if isStatefulSetInSync(current, target) {
		return operations.StepIsCompleted(step, "statefulset already in desired state")
	}

	policy := metav1.DeletePropagationForeground
	if err := e.client.Delete(ctx, current, &client.DeleteOptions{PropagationPolicy: &policy}); err != nil && !apierrors.IsNotFound(err) {
		return operations.StepHasFailed(step, fmt.Errorf("delete statefulset %s for recreate: %w", target.Name, err))
	}
	return operations.StepIsProgressing(
		step,
		fmt.Sprintf("recreating statefulset %s", target.Name),
		statefulSetRecreateRequeue)
}

func (e *StepExecutor) buildTargetStatefulSet(ctx context.Context) (*appsv1.StatefulSet, error) {
	sts, err := e.loadTargetStatefulSet(ctx)
	if err != nil {
		return nil, fmt.Errorf("get target statefulset: %w", err)
	}

	pkiSecret := &corev1.Secret{}
	if err := e.client.Get(
		ctx,
		client.ObjectKey{Namespace: e.tenantIdentity.Namespace, Name: e.tenantIdentity.Namespace + "-pki"},
		pkiSecret,
	); err != nil {
		return nil, fmt.Errorf("get pki secret: %w", err)
	}

	certsChecksum, err := checksum.ComponentCertsChecksum(pkiSecret.Data, e.operation.Spec.Component.PodComponentName())
	if err != nil {
		return nil, fmt.Errorf("calculate certs checksum: %w", err)
	}

	e.applyDesiredChecksums(sts, certsChecksum)

	return sts, nil
}

func (e *StepExecutor) loadTargetStatefulSet(ctx context.Context) (*appsv1.StatefulSet, error) {
	secret := &corev1.Secret{}
	if err := e.client.Get(
		ctx,
		client.ObjectKey{Namespace: e.tenantIdentity.Namespace, Name: e.tenantIdentity.Namespace + constants.VirtualControlPlaneConfigSecretSuffix},
		secret,
	); err != nil {
		return nil, fmt.Errorf("get vcp config secret: %w", err)
	}

	raw, ok := secret.Data[e.operation.Spec.Component.SecretKey()]
	if !ok {
		return nil, fmt.Errorf("component %q not found in config secret", e.operation.Spec.Component)
	}

	rendered := renderComponentManifest(raw, e.operation.Spec.NodeName)

	sts := &appsv1.StatefulSet{}
	if err := yaml.Unmarshal(rendered, sts); err != nil {
		return nil, fmt.Errorf("decode statefulset for %s: %w", e.operation.Spec.Component, err)
	}

	sts.Namespace = e.tenantIdentity.Namespace

	return sts, nil
}

func (e *StepExecutor) applyDesiredChecksums(sts *appsv1.StatefulSet, certsChecksum string) {
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	for k, v := range map[string]string{
		constants.ConfigChecksumAnnotationKey: e.operation.Spec.DesiredConfigChecksum,
		constants.PKIChecksumAnnotationKey:    e.operation.Spec.DesiredPKIChecksum,
		constants.CAChecksumAnnotationKey:     e.operation.Spec.DesiredCAChecksum,
		constants.CertsChecksumAnnotationKey:  certsChecksum,
	} {
		if v != "" {
			sts.Annotations[k] = v
		}
	}
}

func isStatefulSetInSync(current, target *appsv1.StatefulSet) bool {
	for _, key := range []string{
		constants.ConfigChecksumAnnotationKey,
		constants.PKIChecksumAnnotationKey,
		constants.CAChecksumAnnotationKey,
		constants.CertsChecksumAnnotationKey,
	} {
		if target.Annotations[key] != "" && current.Annotations[key] != target.Annotations[key] {
			return false
		}
	}
	return true
}
