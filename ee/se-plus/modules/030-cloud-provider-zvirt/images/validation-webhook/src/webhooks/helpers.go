/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package webhooks adapts the module's admission rules to controller-runtime validators.
package webhooks

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// shouldSkipState reports whether a half-migrated cluster must be let through untouched.
func shouldSkipState(state *zval.State) bool {
	return state != nil && cpapi.ShouldSkipNewModelValidation(state.MigrationStatus)
}

// resultToAdmission turns a validation result into what the API server expects: warnings are
// surfaced to the client either way, errors become a single Invalid status.
func resultToAdmission(result cpvalapi.Result) (admission.Warnings, error) {
	warnings := violationsToAdmissionWarnings(result.Warnings())

	if !result.HasErrors() {
		return warnings, nil
	}

	errors := result.Errors()
	fieldErrors := make(field.ErrorList, 0, len(errors))
	for _, violation := range errors {
		fieldErrors = append(
			fieldErrors,
			field.Invalid(violationFieldPath(violation.Path), violation.Value, violation.Message),
		)
	}

	return warnings, apierrors.NewInvalid(schema.GroupKind{}, "", fieldErrors)
}

func violationsToAdmissionWarnings(violations []cpvalapi.Violation) admission.Warnings {
	if len(violations) == 0 {
		return nil
	}

	warningStrs := make(admission.Warnings, 0, len(violations))
	for _, violation := range violations {
		warningStrs = append(warningStrs, violationMessage(violation))
	}

	return warningStrs
}

func violationMessage(violation cpvalapi.Violation) string {
	if violation.Path == "" {
		return violation.Message
	}

	return violation.Path + ": " + violation.Message
}

func violationMessages(violations []cpvalapi.Violation) []string {
	if len(violations) == 0 {
		return nil
	}

	messages := make([]string, 0, len(violations))
	for _, violation := range violations {
		messages = append(messages, violationMessage(violation))
	}

	return messages
}

// violationFieldPath turns a rule path into a field path. Rule paths name the resource they
// concern ("NodeGroup/master.spec...."), which a field path must not repeat, so the resource
// segment is dropped.
func violationFieldPath(path string) *field.Path {
	if path == "" {
		return field.NewPath("spec")
	}

	parts := strings.Split(path, ".")
	first := parts[0]
	if idx := strings.Index(first, "/"); idx >= 0 {
		first = first[idx+1:]
	}

	fp := field.NewPath(first)
	for _, part := range parts[1:] {
		fp = fp.Child(part)
	}

	return fp
}

// internalBuildError marks a failure of the webhook itself rather than of the reviewed object, so
// the operator is not told their manifest is invalid when it is not.
func internalBuildError(err error) error {
	return apierrors.NewInternalError(fmt.Errorf("build validation state: %w", err))
}

func objectName(obj runtime.Object) string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}

	return accessor.GetName()
}

func objectNamespace(obj runtime.Object) string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}

	return accessor.GetNamespace()
}
