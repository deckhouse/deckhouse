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
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type checksumAnnotations struct {
	ConfigChecksum      string
	PKIChecksum         string
	CAChecksum          string
	CertRenewalID       string
	KubeconfigRenewalID string
}

func checksumAnnotationsFromSpec(spec controlplanev1alpha1.ControlPlaneOperationSpec) checksumAnnotations {
	return checksumAnnotations{
		ConfigChecksum: spec.DesiredConfigChecksum,
		PKIChecksum:    spec.DesiredPKIChecksum,
		CAChecksum:     spec.DesiredCAChecksum,
	}
}

func desiredChecksumAnnotations(spec checksumAnnotations) map[string]string {
	result := make(map[string]string, 5)

	if spec.ConfigChecksum != "" {
		result[constants.ConfigChecksumAnnotationKey] = spec.ConfigChecksum
	}
	if spec.PKIChecksum != "" {
		result[constants.PKIChecksumAnnotationKey] = spec.PKIChecksum
	}
	if spec.CAChecksum != "" {
		result[constants.CAChecksumAnnotationKey] = spec.CAChecksum
	}
	if spec.CertRenewalID != "" {
		result[constants.CertRenewalIDAnnotationKey] = spec.CertRenewalID
	}
	if spec.KubeconfigRenewalID != "" {
		result[constants.KubeconfigRenewalIDAnnotationKey] = spec.KubeconfigRenewalID
	}

	return result
}

func buildSyncManifestAnnotations(op *controlplanev1alpha1.ControlPlaneOperation) checksumAnnotations {
	annotations := checksumAnnotationsFromSpec(op.Spec)

	if stepWasRenewed(op, controlplanev1alpha1.StepRenewPKICerts) {
		annotations.CertRenewalID = op.Name
	}
	if stepWasRenewed(op, controlplanev1alpha1.StepRenewKubeconfigs) {
		annotations.KubeconfigRenewalID = op.Name
	}

	return annotations
}

func stepWasRenewed(op *controlplanev1alpha1.ControlPlaneOperation, step controlplanev1alpha1.StepName) bool {
	cond := op.GetCondition(controlplanev1alpha1.StepConditionType(step))
	if cond == nil {
		return false
	}
	return cond.Status == metav1.ConditionTrue &&
		cond.Reason == controlplanev1alpha1.CPOReasonStepCompleted &&
		cond.Message == controlplanev1alpha1.CPOStepResultRenewed
}
