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
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

func shouldSkipState(state *cpval.State) bool {
	return state != nil && cpapi.ShouldSkipNewModelValidation(state.MigrationStatus)
}

func resultToAdmission(result cpval.Result) (admission.Warnings, error) {
	warnings := violationsToAdmissionWarnings(result.Warnings())

	if !result.HasErrors() {
		return warnings, nil
	}

	errors := result.Errors()
	fieldErrors := make(field.ErrorList, 0, len(errors))
	for _, violation := range errors {
		fieldErrors = append(fieldErrors, field.Invalid(violationFieldPath(violation.Path), nil, violation.Message))
	}

	return warnings, apierrors.NewInvalid(schema.GroupKind{}, "", fieldErrors)
}

func violationsToAdmissionWarnings(violations []cpval.Violation) admission.Warnings {
	if len(violations) == 0 {
		return nil
	}

	warningStrs := make(admission.Warnings, 0, len(violations))
	for _, violation := range violations {
		warningStrs = append(warningStrs, violationMessage(violation))
	}

	return warningStrs
}

func violationMessage(violation cpval.Violation) string {
	if violation.Path == "" {
		return violation.Message
	}

	return violation.Path + ": " + violation.Message
}

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
