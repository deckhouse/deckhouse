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
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// AuthorsPath is the path of the project mutating webhook that sets the author annotations.
const AuthorsPath = "/mutate/v1alpha3/projects/authors"

// RegisterAuthors installs the project mutating webhook that sets the
// projects.deckhouse.io/created-by and projects.deckhouse.io/last-modified-by annotations.
func RegisterAuthors(runtimeManager manager.Manager) {
	runtimeManager.GetWebhookServer().Register(AuthorsPath, &webhook.Admission{Handler: authorsMutator{}})
}

// authorsMutator sets the author annotations of a project to what expectedAuthors says, whatever the
// request carries. A successful call restores removed annotations and overwrites forged values.
type authorsMutator struct{}

func (authorsMutator) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}
	// The webhook configuration skips the controller and Deckhouse already; this is the handler-level
	// backstop, so the platform never records itself as an author.
	if isPrivileged(req.UserInfo.Username) {
		return admission.Allowed("")
	}
	old, updated, err := decodeProjects(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	expected := expectedAuthors(req, old, updated)

	annotations, _, err := unstructured.NestedStringMap(updated, "metadata", "annotations")
	if err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("read the annotations of the project: %w", err))
	}
	if annotations == nil {
		annotations = make(map[string]string, 2)
	}
	setOrDelete(annotations, v1alpha3.ProjectAnnotationCreatedBy, expected.createdBy)
	setOrDelete(annotations, v1alpha3.ProjectAnnotationLastModifiedBy, expected.lastModifiedBy)
	if len(annotations) == 0 {
		unstructured.RemoveNestedField(updated, "metadata", "annotations")
	} else if err := unstructured.SetNestedStringMap(updated, annotations, "metadata", "annotations"); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("set the annotations of the project: %w", err))
	}

	patched, err := json.Marshal(updated)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("encode the project: %w", err))
	}
	// The patch is computed against the raw object, which is decoded as a map rather than a Project,
	// so a field this version of the controller does not know is not removed by the patch.
	return admission.PatchResponseFromRaw(req.Object.Raw, patched)
}

// authors are the target values of the project author annotations for the mutating webhook.
// An empty value means the annotation is absent.
type authors struct {
	createdBy      string
	lastModifiedBy string
	// modifiedNow reports that lastModifiedBy names the requester of this request.
	modifiedNow bool
}

// expectedAuthors returns the author annotations of a project after a create or an update: the
// requester on a create; on an update, the creator kept from the stored project and the requester as
// the last to modify it when the spec, the labels or the other annotations change, the stored value
// otherwise. A write of the status, of the finalizers or of the owner references alone is not a change.
func expectedAuthors(req admission.Request, old, updated map[string]any) authors {
	user := req.UserInfo.Username
	if req.Operation == admissionv1.Create {
		return authors{createdBy: user, lastModifiedBy: user, modifiedNow: true}
	}
	stored := stringAnnotations(old)
	expected := authors{
		createdBy:      stored[v1alpha3.ProjectAnnotationCreatedBy],
		lastModifiedBy: stored[v1alpha3.ProjectAnnotationLastModifiedBy],
	}
	if contentChanged(old, updated) {
		expected.lastModifiedBy = user
		expected.modifiedNow = true
	}
	return expected
}

// contentChanged reports whether an update changes the spec, the labels or the annotations of a
// project. The author annotations are what the change sets, and kubectl rewrites its last applied
// configuration on an apply that may change nothing else, so neither counts.
func contentChanged(old, updated map[string]any) bool {
	oldSpec, _, _ := unstructured.NestedFieldNoCopy(old, "spec")
	newSpec, _, _ := unstructured.NestedFieldNoCopy(updated, "spec")
	if !equality.Semantic.DeepEqual(emptyAsNil(oldSpec), emptyAsNil(newSpec)) {
		return true
	}
	if !maps.Equal(stringLabels(old), stringLabels(updated)) {
		return true
	}
	oldAnnotations, newAnnotations := stringAnnotations(old), stringAnnotations(updated)
	for _, key := range []string{
		v1alpha3.ProjectAnnotationCreatedBy,
		v1alpha3.ProjectAnnotationLastModifiedBy,
		corev1.LastAppliedConfigAnnotation,
	} {
		delete(oldAnnotations, key)
		delete(newAnnotations, key)
	}
	return !maps.Equal(oldAnnotations, newAnnotations)
}

// authorsRefusal checks the author annotations against the current request and the stored project.
// A failed mutating call is ignored, and SSA may remove created-by even without a content change.
// AdmissionRequest carries no indication of that failure, so an absent created-by is accepted
// regardless of why it is missing. A supplied created-by must still match the expected creator.
// A value that would record this request as the modifier may be absent, and a content update may
// still carry the previously stored modifier.
func authorsRefusal(req admission.Request) (string, error) {
	old, updated, err := decodeProjects(req)
	if err != nil {
		return "", err
	}
	expected := expectedAuthors(req, old, updated)
	priorModifier, hadPriorModifier := stringAnnotations(old)[v1alpha3.ProjectAnnotationLastModifiedBy]
	actual := stringAnnotations(updated)
	name, _, _ := unstructured.NestedString(updated, "metadata", "name")
	for _, check := range []struct {
		key         string
		expected    string
		mayBeAbsent bool
	}{
		{key: v1alpha3.ProjectAnnotationCreatedBy, expected: expected.createdBy, mayBeAbsent: true},
		{key: v1alpha3.ProjectAnnotationLastModifiedBy, expected: expected.lastModifiedBy, mayBeAbsent: expected.modifiedNow},
	} {
		value, ok := actual[check.key]
		if !ok && check.mayBeAbsent {
			continue
		}
		if check.key == v1alpha3.ProjectAnnotationLastModifiedBy && expected.modifiedNow &&
			ok && hadPriorModifier && value == priorModifier {
			continue
		}
		if value == check.expected && ok == (check.expected != "") {
			continue
		}
		return fmt.Sprintf("the %s annotation of the %q project is set by the multitenancy-manager webhook and cannot be set or removed",
			check.key, name), nil
	}
	return "", nil
}

// decodeProjects decodes, as maps, the stored project of an update (nil on a create) and the project
// of the request, so that a field the Project type does not know is compared and kept as well.
func decodeProjects(req admission.Request) (map[string]any, map[string]any, error) {
	var updated map[string]any
	if err := json.Unmarshal(req.Object.Raw, &updated); err != nil {
		return nil, nil, fmt.Errorf("decode the project: %w", err)
	}
	if req.Operation != admissionv1.Update {
		return nil, updated, nil
	}
	var old map[string]any
	if err := json.Unmarshal(req.OldObject.Raw, &old); err != nil {
		return nil, nil, fmt.Errorf("decode the stored project: %w", err)
	}
	return old, updated, nil
}

// isPrivileged reports whether the user is the controller or Deckhouse, the two identities the project
// webhooks trust.
func isPrivileged(username string) bool {
	return username == rolebindingwebhook.ControllerServiceAccount || username == rolebindingwebhook.DeckhouseServiceAccount
}

func stringLabels(obj map[string]any) map[string]string {
	labels, _, _ := unstructured.NestedStringMap(obj, "metadata", "labels")
	return labels
}

// stringAnnotations returns a copy of the annotations of the object, nil when it has none.
func stringAnnotations(obj map[string]any) map[string]string {
	annotations, _, _ := unstructured.NestedStringMap(obj, "metadata", "annotations")
	return annotations
}

// emptyAsNil makes an absent spec and an empty one equal.
func emptyAsNil(value any) any {
	if m, ok := value.(map[string]any); ok && len(m) == 0 {
		return nil
	}
	return value
}

func setOrDelete(annotations map[string]string, key, value string) {
	if value == "" {
		delete(annotations, key)
		return
	}
	annotations[key] = value
}
