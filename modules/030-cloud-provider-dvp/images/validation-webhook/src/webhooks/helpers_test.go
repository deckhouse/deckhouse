// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

func TestShouldSkipState(t *testing.T) {
	t.Parallel()

	if !shouldSkipState(&cpval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}) {
		t.Fatal("shouldSkipState(pending migration) = false, want true")
	}

	if shouldSkipState(&cpval.State{}) {
		t.Fatal("shouldSkipState(empty) = true, want false")
	}

	if shouldSkipState(nil) {
		t.Fatal("shouldSkipState(nil) = true, want false")
	}
}

func TestResultToAdmission(t *testing.T) {
	t.Parallel()

	warnings, err := resultToAdmission(cpval.Result{})
	if err != nil || warnings != nil {
		t.Fatalf("resultToAdmission() = (%v, %v), want (nil, nil)", warnings, err)
	}

	withWarnings := cpval.Result{}
	withWarnings.AddWarning("spec.zone", "deprecated_zone", nil, "zone is deprecated")
	warnings, err = resultToAdmission(withWarnings)
	if err != nil {
		t.Fatalf("resultToAdmission() error = %v, want nil for warnings-only result", err)
	}
	if len(warnings) != 1 || warnings[0] != "spec.zone: zone is deprecated" {
		t.Fatalf("resultToAdmission() warnings = %v, want formatted warning", warnings)
	}

	denied := cpval.Result{}
	denied.AddError("", "denied", nil, "denied")
	warnings, err = resultToAdmission(denied)
	if err == nil {
		t.Fatal("resultToAdmission() error = nil, want denial")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("resultToAdmission() error = %T, want Invalid", err)
	}
	if warnings != nil {
		t.Fatalf("resultToAdmission() warnings = %v, want nil when only errors are present", warnings)
	}

	deniedWithWarnings := cpval.Result{}
	deniedWithWarnings.AddError("spec.enabled", "disabled", nil, "module must be enabled")
	deniedWithWarnings.AddWarning("spec.settings", "legacy_setting", nil, "setting is deprecated")
	warnings, err = resultToAdmission(deniedWithWarnings)
	if err == nil || !apierrors.IsInvalid(err) {
		t.Fatalf("resultToAdmission() error = %v, want Invalid", err)
	}
	if len(warnings) != 1 || warnings[0] != "spec.settings: setting is deprecated" {
		t.Fatalf("resultToAdmission() warnings = %v, want warnings preserved on denial", warnings)
	}

	deniedWithValue := cpval.Result{}
	deniedWithValue.AddError(
		"Secret/d8-credentials.data.authScheme",
		"unsupported_auth_scheme",
		"apiToken",
		`authScheme "apiToken" is not allowed`,
	)
	_, err = resultToAdmission(deniedWithValue)
	if err == nil || !apierrors.IsInvalid(err) {
		t.Fatalf("resultToAdmission() error = %v, want Invalid", err)
	}
	statusErr, ok := err.(*apierrors.StatusError)
	if !ok {
		t.Fatalf("resultToAdmission() error = %T, want *apierrors.StatusError", err)
	}
	details := statusErr.Status().Details
	if details == nil || len(details.Causes) != 1 {
		t.Fatalf("Status().Details.Causes = %#v, want one field error", details)
	}
	if details.Causes[0].Field != "d8-credentials.data.authScheme" {
		t.Fatalf("field error path = %q, want d8-credentials.data.authScheme", details.Causes[0].Field)
	}
	if !strings.Contains(details.Causes[0].Message, `Invalid value: "apiToken"`) {
		t.Fatalf("field error message = %q, want Invalid value apiToken", details.Causes[0].Message)
	}
	if !strings.Contains(details.Causes[0].Message, `authScheme "apiToken" is not allowed`) {
		t.Fatalf("field error message = %q, want authScheme denial text", details.Causes[0].Message)
	}
}

func TestValidateAdmissionStateRunsOnlyInvariants(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		ModuleName: dvpval.ModuleName,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: dvpval.ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: boolPtr(true),
				Version: 2,
			},
		},
	}
	result := dvpval.ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("validateAdmissionState() = %q, want only invariants without preflight requirements", result.Error())
	}
}

func TestValidateAdmissionStateDoesNotEnforceMasterTopology(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		ModuleName: dvpval.ModuleName,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: dvpval.ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: boolPtr(true),
				Version: 2,
			},
		},
	}
	if strings.Contains(dvpval.ValidateInvariants(state).Error(), `NodeGroup "master" is required`) {
		t.Fatal("validateAdmissionState() enforced preflight master topology")
	}
}

func TestObjectNameAndNamespace(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: dvpval.Namespace}}
	if objectName(secret) != "s" || objectNamespace(secret) != dvpval.Namespace {
		t.Fatalf("objectName/Namespace() = (%q, %q)", objectName(secret), objectNamespace(secret))
	}
	if objectName(&metav1.Status{}) != "" {
		t.Fatal("objectName() without metadata = non-empty")
	}
}

func boolPtr(value bool) *bool {
	return &value
}
