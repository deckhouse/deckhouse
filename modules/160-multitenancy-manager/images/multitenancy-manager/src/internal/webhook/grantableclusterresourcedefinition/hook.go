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

// Package grantableclusterresourcedefinition validates GrantableClusterResourceDefinition objects:
//
//   - spec.grantedResource must be a cluster-scoped kind. The catalog reconciler refuses a namespaced
//     one for good (resolve.ErrNamespacedGrantedResource) and the definition reconciler reports it as
//     GrantedResourceValid=False; this webhook refuses it at apply time. A kind the REST mapper does
//     not know (its CRD is not installed yet) has no scope to judge and is let through, as is a kind
//     whose mapping fails for another reason (logged): the reconciler reports both later. The mapper
//     is the grants REST mapper, which rediscovers the API only after its reset (every ResyncInterval
//     and on a list 404), so a request naming a made-up group cannot make it rediscover at will.
//   - every catalogFields path must be one the catalog projection reads (it compiles, it is a singular
//     query, it does not overlap a forbidden path), and a value-backed definition, which has no
//     objects to read, may not declare any. The rules live in internal/engine (DefinitionProblems),
//     shared with the definition reconciler that reports them as CatalogFieldsValid. The projection
//     skips an unusable entry without a trace; this webhook turns it into a refusal at apply time,
//     while the author still has the object in front of them.
//
// On UPDATE the scope is re-checked only when spec.grantedResource changed, only catalogFields entries
// that are new or changed relative to the old object have their paths checked, and the value-backed
// rule is re-checked only when spec.catalogFields or spec.grantedResource changed; an object that is
// being deleted is not checked at all: an object stored before this webhook existed, or while it was
// unavailable (failurePolicy: Ignore), must stay editable — metadata edits, finalizer removal — instead
// of being refused on every write.
package grantableclusterresourcedefinition

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/resolve"
)

// Register installs the GrantableClusterResourceDefinition validating webhook. factory must be the one
// the catalog projection uses, so a path is accepted here exactly when it is projected there; mapper
// the grants REST mapper the catalog and definition reconcilers resolve with, so a kind is refused here
// exactly when it is there. A kind whose CRD was installed after the last reset of the mapper is unknown
// here until the next one (at most ResyncInterval) and is let through; the definition reconciler
// reports it.
func Register(runtimeManager manager.Manager, factory jsonpath.Factory, mapper meta.RESTMapper) {
	hook := &webhook.Admission{Handler: &validator{factory: factory, mapper: mapper}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha1/grantableclusterresourcedefinitions", hook)
}

// validator needs no client: the rules are properties of the object (and its previous version) and of
// the scope of its kind, which the REST mapper knows.
type validator struct {
	factory jsonpath.Factory
	mapper  meta.RESTMapper
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	def := new(grantsv1alpha1.GrantableClusterResourceDefinition)
	if err := yaml.Unmarshal(req.Object.Raw, def); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	// Deletion must always be able to finish, e.g. by removing a finalizer.
	if def.DeletionTimestamp != nil {
		return admission.Allowed("")
	}

	// Without an old object there is nothing to ratchet against: everything is checked.
	var old *grantsv1alpha1.GrantableClusterResourceDefinition
	if req.Operation == admissionv1.Update && len(req.OldObject.Raw) > 0 {
		old = new(grantsv1alpha1.GrantableClusterResourceDefinition)
		if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	var problems []string
	if old == nil || !reflect.DeepEqual(old.Spec.GrantedResource, def.Spec.GrantedResource) {
		problem, err := resolve.GrantedResourceProblem(v.mapper, def)
		switch {
		case problem != "":
			problems = append(problems, problem)
		case err != nil && !meta.IsNoMatchError(err):
			// Discovery failed: the scope is unknown, and a transient failure must not refuse the apply.
			ctrllog.FromContext(ctx).Error(err, "cannot tell the scope of grantedResource, not checked", "definition", def.Name)
		}
	}
	if old == nil {
		problems = append(problems, engine.DefinitionProblems(v.factory, def)...)
	} else {
		// Ratcheting, per check. Same order of problems as engine.DefinitionProblems.
		fieldsChanged := !slices.Equal(old.Spec.CatalogFields, def.Spec.CatalogFields)
		if fieldsChanged || !reflect.DeepEqual(old.Spec.GrantedResource, def.Spec.GrantedResource) {
			if p := engine.ValueBackedCatalogFieldsProblem(def); p != "" {
				problems = append(problems, p)
			}
		}
		for i, f := range def.Spec.CatalogFields {
			// An entry the old object already had was accepted (or predates this webhook) and is not
			// re-judged. Compared by content: indexes shift on insert and delete.
			if slices.Contains(old.Spec.CatalogFields, f) {
				continue
			}
			if p := engine.CatalogFieldProblem(v.factory, i, f); p != "" {
				problems = append(problems, p)
			}
		}
	}
	if len(problems) > 0 {
		return admission.Denied(fmt.Sprintf("the '%s' GrantableClusterResourceDefinition is invalid: %s",
			def.Name, strings.Join(problems, "; ")))
	}
	return admission.Allowed("")
}
