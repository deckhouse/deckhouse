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

// Package grantableclusterresourcereference validates GrantableClusterResourceReference objects:
// every path an entry carries must compile, a path that asks for defaulting (FillEmpty/Coerce) must be
// one the defaulting webhook can actually write to, every entry's scope must lie within spec.rule, and
// every request spec.rule matches must select an entry. The rules live in internal/engine
// (ReferenceProblems), shared with the reference reconciler that reports them as FieldPathsValid.
//
// /is-granted and /defaults read the paths of every stored reference on every admission of a usage
// object. A path that does not compile cannot be evaluated: /is-granted logs it and skips the
// reference (failing the request instead, under failurePolicy: Fail, would block every CREATE/UPDATE
// of the resources in the reference's rule in every project), so the reference checks nothing. Defaulting needs a single, unambiguous location to write a JSON
// Patch at and therefore accepts simple member paths only; a wildcard path with defaulting: FillEmpty
// used to be accepted, bind and then never default anything, with nothing to explain it. A scope typo
// (pod, Pods) or entries that do not cover the whole rule leave requests for which no entry applies,
// and all three consumers skip those without a check: a silent fail-open. This webhook turns all of it
// into a refusal at apply time, while the author still has the object in front of them.
//
// On UPDATE only entries that are new or changed relative to the old object have their paths checked,
// an entry's scope is re-checked when the entry or spec.rule changed, and coverage when spec.rule or
// spec.fieldPaths changed; an object that is being deleted is not checked at all: an object stored before this webhook existed,
// or while it was unavailable (failurePolicy: Ignore), must stay editable — metadata edits, finalizer
// removal — instead of being refused on every write.
package grantableclusterresourcereference

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
)

// Register installs the GrantableClusterResourceReference validating webhook. factory must be the one
// /is-granted and /defaults use, so a path compiles here exactly when it compiles there.
func Register(runtimeManager manager.Manager, factory jsonpath.Factory) {
	hook := &webhook.Admission{Handler: &validator{factory: factory}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha1/grantableclusterresourcereferences", hook)
}

// validator needs no client: the rule is a property of the object (and its previous version) alone.
type validator struct {
	factory jsonpath.Factory
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	reference := new(grantsv1alpha1.GrantableClusterResourceReference)
	if err := yaml.Unmarshal(req.Object.Raw, reference); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	// Deletion must always be able to finish, e.g. by removing a finalizer.
	if reference.DeletionTimestamp != nil {
		return admission.Allowed("")
	}

	// Without an old object there is nothing to ratchet against: everything is checked.
	var old *grantsv1alpha1.GrantableClusterResourceReferenceSpec
	if req.Operation == admissionv1.Update && len(req.OldObject.Raw) > 0 {
		oldObject := new(grantsv1alpha1.GrantableClusterResourceReference)
		if err := yaml.Unmarshal(req.OldObject.Raw, oldObject); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		old = &oldObject.Spec
	}
	spec := reference.Spec

	var problems []string
	if old == nil {
		problems = engine.ReferenceProblems(v.factory, spec)
	} else {
		// Ratcheting, per check. Same order of problems as engine.ReferenceProblems.
		ruleUnchanged := reflect.DeepEqual(old.Rule, spec.Rule)
		for i, fp := range spec.FieldPaths {
			// An entry the old object already had, byte for byte, was accepted (or predates this
			// webhook) and its paths are not re-judged. Compared by content: indexes shift on insert
			// and delete.
			entryUnchanged := slices.ContainsFunc(old.FieldPaths, func(o grantsv1alpha1.FieldPath) bool { return reflect.DeepEqual(o, fp) })
			if !entryUnchanged {
				problems = append(problems, engine.EntryPathProblems(v.factory, i, fp)...)
			}
			// The scope is judged against the rule, so a rule change re-judges every scope: otherwise
			// dropping a resource from the rule would leave a stored entry scoped to it unnoticed.
			if !entryUnchanged || !ruleUnchanged {
				problems = append(problems, engine.EntryScopeProblems(spec.Rule, i, fp)...)
			}
		}
		// Coverage is a property of the whole spec, not of one entry: it is judged whenever the rule
		// or the entries change, and never on a metadata-only edit of a stored object.
		if !ruleUnchanged || !reflect.DeepEqual(old.FieldPaths, spec.FieldPaths) {
			problems = append(problems, engine.CoverageProblems(spec.Rule, spec.FieldPaths)...)
		}
	}
	if len(problems) > 0 {
		return admission.Denied(fmt.Sprintf("the '%s' GrantableClusterResourceReference is invalid: %s",
			reference.Name, strings.Join(problems, "; ")))
	}
	return admission.Allowed("")
}
