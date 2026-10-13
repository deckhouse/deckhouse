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

package project

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"controller/apis/deckhouse.io/v1alpha3"
)

// refusedManifestKind is a kind of which the manifests of a template may not write some objects.
type refusedManifestKind struct {
	schema.GroupKind
	// resource is the resource of the kind, the name the admission policy matches.
	resource string
	// refuses mirrors the validation of the kind in the policy: it reports whether the API server
	// refuses the apply of rendered as the user of the project, as a create when live is nil and as an
	// update of live otherwise.
	refuses func(rendered, live *unstructured.Unstructured) bool
	// which says which objects of the kind are refused, in the words of a warning that follows the
	// name of one.
	which string
	// instead says what to use in place of such an object, in the words of a warning.
	instead string
}

// adminAccessLevel is the access level of an AuthorizationRule that the manifests may not grant.
const adminAccessLevel = "Admin"

// refusedManifestKinds are the kinds of which the d8-multitenancy-manager-manifests admission policy
// refuses some objects to the user of a project (templates/validation.yaml of the module). The policy
// does the refusing; the list is here so that the Project webhook warns of such an object before the
// ManifestsApplied condition names the refusal. A test keeps the kinds and the operations of the two
// the same, and each predicate is written after the expression of its kind.
var refusedManifestKinds = []refusedManifestKind{
	{
		GroupKind: schema.GroupKind{Group: "deckhouse.io", Kind: "AuthorizationRule"},
		resource:  "authorizationrules",
		// The administrators of a project are set in .spec.administrators of the Project. An update
		// that keeps a rule of the Admin level passes, so that a rule a switch from a Helm text left in
		// place keeps working once the manifests take it over, but only while its subjects stay the
		// same or fewer: adding one would make a new administrator.
		refuses: func(rendered, live *unstructured.Unstructured) bool {
			return grantsAdmin(rendered) && (live == nil || !grantsAdmin(live) || !subjectsWithin(rendered, live))
		},
		which:   "of the Admin level",
		instead: "set the administrators of the project in .spec.administrators of the Project",
	},
}

// grantsAdmin reports whether an AuthorizationRule grants the Admin access level. A rule without
// spec.accessLevel, or with one that is not a string, does not, as the policy has it.
// subjectsWithin tells whether every subject of the rendered rule is a subject of the live one, by
// kind, name and namespace (ruleSubjects), the fields the kind keeps of a subject.
func subjectsWithin(rendered, live *unstructured.Unstructured) bool {
	have := make(map[subjectKey]struct{})
	for _, subject := range ruleSubjects(live) {
		have[subject] = struct{}{}
	}
	for _, subject := range ruleSubjects(rendered) {
		if _, ok := have[subject]; !ok {
			return false
		}
	}
	return true
}

func grantsAdmin(rule *unstructured.Unstructured) bool {
	level, found, err := unstructured.NestedString(rule.Object, "spec", "accessLevel")
	return err == nil && found && level == adminAccessLevel
}

// RefusedManifestObject is an object of a render of the manifests that the API server refuses to
// create as the user of the project. RefusedManifestObjects makes it.
type RefusedManifestObject struct {
	// Object is the object as the render has it, in the namespace the controller puts it in.
	Object *unstructured.Unstructured
	kind   refusedManifestKind
}

// RefusesUpdateOf reports whether the API server refuses the apply of the object as an update of
// live, the object that exists under its kind, namespace and name.
func (o RefusedManifestObject) RefusesUpdateOf(live *unstructured.Unstructured) bool {
	return o.kind.refuses(o.Object, live)
}

// String names the object, says which objects of its kind are refused and what to use instead.
func (o RefusedManifestObject) String() string {
	return fmt.Sprintf("%s %s/%s %s, %s", o.Object.GetKind(), o.Object.GetNamespace(), o.Object.GetName(), o.kind.which, o.kind.instead)
}

// RefusedManifestObjects lists the objects of a render of the manifests that the API server refuses to
// create as the user of the project, in the order the render has them. An object without a namespace
// gets the main namespace of the project, where the controller puts it. The apply of an object that
// exists is an update, which the policy refuses only when it makes a refused object out of the live
// one, so the caller reads each listed object and, when it exists, asks RefusesUpdateOf:
// status.manifestObjects of the project does not tell, as an object goes on that list before its
// apply, a refused one included. A render that does not decode is listed up to the document that
// fails, and the ManifestsApplied condition says what is wrong with it.
func RefusedManifestObjects(project *v1alpha3.Project, rendered string) []RefusedManifestObject {
	var refused []RefusedManifestObject
	// The error is the one the controller reports in the ManifestsApplied condition, and the objects
	// before it are worth the warning all the same.
	_ = eachManifestObject(rendered, func(object *unstructured.Unstructured) error {
		groupKind := object.GroupVersionKind().GroupKind()
		for _, kind := range refusedManifestKinds {
			if kind.GroupKind != groupKind || !kind.refuses(object, nil) {
				continue
			}
			object.SetNamespace(keyOf(releaseObjectOf(object), project.Name).namespace)
			refused = append(refused, RefusedManifestObject{Object: object, kind: kind})
		}
		return nil
	})
	return refused
}
