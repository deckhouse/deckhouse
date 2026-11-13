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

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/resolve"
)

var _ http.Handler = &DefaultsMutator{}

// DefaultsMutator is the /defaults mutating webhook: on CREATE it injects the per-project default
// granted name into a referenced field, per the reference path's defaulting mode (FillEmpty/Coerce).
type DefaultsMutator struct {
	log     logr.Logger
	cl      client.Reader
	mapper  meta.RESTMapper
	factory jsonpath.Factory
}

// NewDefaultsMutator builds the /defaults mutating webhook. cl must never wait for an informer sync,
// for the same reason as in the /is-granted validator, and it gets the same reader (see
// NewIsGrantedValidator).
func NewDefaultsMutator(log logr.Logger, cl client.Reader, mapper meta.RESTMapper, factory jsonpath.Factory) *DefaultsMutator {
	return &DefaultsMutator{log: log.WithValues("component", "defaults"), cl: cl, mapper: mapper, factory: factory}
}

// InstallInto registers the handler on the webhook server.
func (m *DefaultsMutator) InstallInto(srv webhook.Server) { srv.Register("/defaults", m) }

func (m *DefaultsMutator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := &admissionv1.AdmissionReview{}
	if err := decodeReview(w, r, review); err != nil {
		http.Error(w, "invalid AdmissionReview: "+err.Error(), http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		http.Error(w, "AdmissionReview without request", http.StatusBadRequest)
		return
	}
	req := review.Request
	// Hard-bound the decision so the webhook always answers quickly and can never become a queue lock.
	ctx, cancel := context.WithTimeout(r.Context(), webhookDecisionTimeout)
	defer cancel()

	resp, err := m.decide(ctx, req)
	if err != nil {
		m.log.Error(err, "defaults decision failed")
		http.Error(w, "defaults decision failed", http.StatusInternalServerError)
		return
	}
	review.Response = resp
	writeReview(w, review)
}

func (m *DefaultsMutator) decide(ctx context.Context, req *admissionv1.AdmissionRequest) (*admissionv1.AdmissionResponse, error) {
	// Never mutate (nor stall) a system / cluster-component / module writer: a module's resources are
	// applied into project namespaces by the deckhouse-controller's Helm release, and with
	// failurePolicy: Fail a slow/erroring defaulting call fails that apply and deadlocks the module's
	// queue. The apiserver-level matchConditions already skip this webhook for those writers; this is
	// the handler-level backstop. Same isSystemRequest set as protect.go; /is-granted uses the
	// narrower isAutomatedSystemWriter instead.
	if isSystemRequest(req) {
		return allowedResponse(req.UID), nil
	}
	if namespaces.IsSystem(req.Namespace) || req.SubResource != "" || req.Namespace == "" {
		return allowedResponse(req.UID), nil
	}
	// Defaulting only on CREATE: never re-inject on UPDATE (a user may intentionally clear a field).
	if req.Operation != admissionv1.Create || len(req.Object.Raw) == 0 {
		return allowedResponse(req.UID), nil
	}

	group, version, resourcePlural := req.Resource.Group, req.Resource.Version, req.Resource.Resource
	refs, err := resolve.ReferencesForRequest(ctx, m.cl, group, version, resourcePlural)
	if err != nil {
		return nil, fmt.Errorf("references: %w", err)
	}
	// Defaulting is per path: None never fills in (opt-in toggle annotations stay absent). A request
	// none of whose references may default anything is answered before the object is even decoded.
	type candidate struct {
		mr resolve.MatchedReference
		fp v1alpha1.FieldPath
	}
	defaulting := make([]resolve.MatchedReference, 0, len(refs))
	for _, mr := range refs {
		if mayDefault(mr.Reference.Spec.FieldPaths, group, version, resourcePlural) {
			defaulting = append(defaulting, mr)
		}
	}
	if len(defaulting) == 0 {
		return allowedResponse(req.UID), nil
	}

	obj := map[string]any{}
	if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	active := make([]candidate, 0, len(defaulting))
	for _, mr := range defaulting {
		fieldPaths := mr.Reference.Spec.FieldPaths
		idx, err := engine.ApplicableFieldPathIndex(m.factory, fieldPaths, group, version, resourcePlural, obj)
		if err != nil || idx < 0 || !engine.DefaultingActive(fieldPaths[idx]) {
			continue
		}
		active = append(active, candidate{mr: mr, fp: fieldPaths[idx]})
	}

	// The fields that may be patched are found first; the namespace, its project and the grants are read
	// only when there is one. FillEmpty patches only an empty field, so a workload that names its value
	// itself costs no read beyond the references.
	type target struct {
		candidate
		segs  []string
		value string
	}
	var targets []target
	for _, c := range active {
		// A path this parser cannot handle is not defaultable. The GrantableClusterResourceReference
		// validating webhook refuses such an entry at apply time, but it is not a guarantee: the object
		// may be older than the webhook, or have been written while the webhook was unavailable
		// (failurePolicy: Ignore). Skipping keeps validation working while defaulting stays off.
		segs, ok := engine.ParsePathSegments(m.factory, c.fp.Path)
		if !ok {
			continue
		}
		parentOK, value := fieldState(obj, segs)
		if !parentOK {
			// A parent object is missing, so a JSON Patch "add" would be unsafe.
			continue
		}
		// A set value is rewritten under Coerce only; FillEmpty leaves it as it is.
		if value != "" && c.fp.Defaulting != v1alpha1.DefaultingCoerce {
			continue
		}
		targets = append(targets, target{candidate: c, segs: segs, value: value})
	}
	if len(targets) == 0 {
		return allowedResponse(req.UID), nil
	}

	ns := &corev1.Namespace{}
	if err := m.cl.Get(ctx, client.ObjectKey{Name: req.Namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return allowedResponse(req.UID), nil
		}
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	grants, err := resolve.GrantsForNamespace(ctx, m.cl, ns)
	if err != nil {
		return nil, fmt.Errorf("grants: %w", err)
	}

	unresolvable := map[string]struct{}{}

	var patches []jsonPatchOperation
	var warnings []string
	for _, t := range targets {
		mr, fp, segs, value := t.mr, t.fp, t.segs, t.value
		def := mr.Definition
		if _, skip := unresolvable[def.Name]; skip {
			continue
		}
		// Only the current value and the default are looked at (ResolveNames), not every granted object.
		resolved, err := resolve.ResolveNames(ctx, m.cl, m.mapper, def, resolve.EntriesFor(grants, def.Name), []string{value})
		if resolve.IsConfigurationError(err) {
			// Inert, as in /is-granted: a definition that cannot be resolved for a configuration
			// reason defaults nothing instead of failing every CREATE of the reference's rule in every
			// project under failurePolicy: Fail. Any other Resolve error still fails the request.
			m.log.Error(err, "skipping reference: definition cannot be resolved",
				"namespace", req.Namespace, "name", req.Name,
				"definition", def.Name, "reference", mr.Reference.Name)
			unresolvable[def.Name] = struct{}{}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", def.Name, err)
		}
		defName := resolved.Default()
		if defName == "" {
			// No project default configured: leave the field as-is; /is-granted rejects a bad value.
			continue
		}

		// FillEmpty injects only into an empty field. Coerce also rewrites a non-empty value that is not
		// available to the project (for fields a built-in admission controller pre-fills).
		shouldDefault := value == "" ||
			(fp.Defaulting == v1alpha1.DefaultingCoerce && !resolved.IsAvailable(value))
		if shouldDefault {
			patches = append(patches, jsonPatchOperation{Op: "add", Path: jsonPointer(segs), Value: defName})
			if value != "" {
				// The author asked for something and got something else; say so on the response the
				// author reads (kubectl prints admission warnings), the way USAGE describes Coerce.
				warnings = append(warnings, fmt.Sprintf(
					"[multitenancy] %s %q: %s %q is not available to project %q and was replaced with the project default %q",
					req.Kind.Kind, req.Name, fp.Path, value, resolve.ProjectName(ns), defName))
			}
		}
	}

	if len(patches) == 0 {
		return allowedResponse(req.UID), nil
	}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("marshal patches: %w", err)
	}
	resp := allowedResponse(req.UID)
	resp.Patch = patchBytes
	resp.PatchType = ptr.To(admissionv1.PatchTypeJSONPatch)
	resp.Warnings = warnings
	return resp, nil
}

// mayDefault reports whether an entry of fieldPaths that may apply to the resource/group/version asks
// for defaulting.
func mayDefault(fieldPaths []v1alpha1.FieldPath, group, version, resource string) bool {
	for i := range fieldPaths {
		if engine.DefaultingActive(fieldPaths[i]) && engine.ScopeMatches(&fieldPaths[i], group, version, resource) {
			return true
		}
	}
	return false
}

// fieldState reports whether all parent objects of the field exist (so a JSON Patch "add" is safe,
// first return) and the field's current string value (second return; empty if the field is absent,
// nil, or not a string).
func fieldState(obj map[string]any, segs []string) (bool, string) {
	cur := obj
	for i := 0; i < len(segs)-1; i++ {
		m, ok := cur[segs[i]].(map[string]any)
		if !ok {
			return false, ""
		}
		cur = m
	}
	v, _ := cur[segs[len(segs)-1]].(string)
	return true, v
}

// jsonPointer builds an RFC6901 JSON Pointer from path segments.
func jsonPointer(segs []string) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1"))
	}
	return b.String()
}
