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

// NewDefaultsMutator builds the /defaults mutating webhook. cl is a direct (uncached) API reader for the
// same reason as the /is-granted validator: a cache-backed read lazily starts an informer and blocks on
// its sync inside the admission request, which can exceed the webhook deadline and pile up into a queue
// lock. A direct reader keeps reads bounded so the webhook cannot hang.
func NewDefaultsMutator(log logr.Logger, cl client.Reader, mapper meta.RESTMapper, factory jsonpath.Factory) *DefaultsMutator {
	return &DefaultsMutator{log: log.WithValues("component", "defaults"), cl: cl, mapper: mapper, factory: factory}
}

// InstallInto registers the handler on the webhook server.
func (m *DefaultsMutator) InstallInto(srv webhook.Server) { srv.Register("/defaults", m) }

func (m *DefaultsMutator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := &admissionv1.AdmissionReview{}
	if err := decodeReview(r, review); err != nil {
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
	// the handler-level backstop. Mirrors is_granted.go / protect.go.
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
	if len(refs) == 0 {
		return allowedResponse(req.UID), nil
	}

	ns := &corev1.Namespace{}
	if err := m.cl.Get(ctx, client.ObjectKey{Name: req.Namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return allowedResponse(req.UID), nil
		}
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	grants, err := resolve.GrantsForLabels(ctx, m.cl, ns.Labels)
	if err != nil {
		return nil, fmt.Errorf("grants: %w", err)
	}

	obj := map[string]any{}
	if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}

	resolvedByDef := map[string]*resolve.Resolved{}
	availByDef := map[string]map[string]bool{}

	var patches []jsonPatchOperation
	for _, mr := range refs {
		fp, ok := engine.SelectFieldPath(mr.Reference.Spec.FieldPaths, group, version)
		if !ok {
			continue
		}
		// Defaulting is per path: None never fills in (opt-in toggle annotations stay absent).
		if fp.Defaulting == "" || fp.Defaulting == v1alpha1.DefaultingNone {
			continue
		}
		guard, err := engine.EvalMatch(m.factory, fp.Match, obj)
		if err != nil || !guard {
			continue
		}
		segs, ok := parsePathSegments(fp.Path)
		if !ok {
			continue
		}
		parentOK, value := fieldState(obj, segs)
		if !parentOK {
			// A parent object is missing, so a JSON Patch "add" would be unsafe.
			continue
		}

		def := mr.Definition
		resolved := resolvedByDef[def.Name]
		if resolved == nil {
			resolved, err = resolve.Resolve(ctx, m.cl, m.mapper, def, resolve.EntriesFor(grants, def.Name))
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", def.Name, err)
			}
			resolvedByDef[def.Name] = resolved
			avail := make(map[string]bool, len(resolved.Available()))
			for _, a := range resolved.Available() {
				avail[a.Name] = true
			}
			availByDef[def.Name] = avail
		}
		defName := resolved.Default()
		if defName == "" {
			// No project default configured: leave the field as-is; /is-granted rejects a bad value.
			continue
		}

		// FillEmpty injects only into an empty field. Coerce also rewrites a non-empty value that is not
		// available to the project (for fields a built-in admission controller pre-fills).
		shouldDefault := value == "" ||
			(fp.Defaulting == v1alpha1.DefaultingCoerce && !availByDef[def.Name][value])
		if shouldDefault {
			patches = append(patches, jsonPatchOperation{Op: "add", Path: jsonPointer(segs), Value: defName})
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
	return resp, nil
}

// parsePathSegments parses a simple member JSONPath ($.a.b['c']) into its segments. It returns
// ok=false for anything with wildcards, indexes or filters (not safely defaultable).
func parsePathSegments(expr string) ([]string, bool) {
	if !strings.HasPrefix(expr, "$") {
		return nil, false
	}
	s := expr[1:]
	var segs []string
	for len(s) > 0 {
		switch s[0] {
		case '.':
			s = s[1:]
			j := 0
			for j < len(s) && s[j] != '.' && s[j] != '[' {
				j++
			}
			if j == 0 {
				return nil, false
			}
			seg := s[:j]
			if strings.ContainsAny(seg, "*?[]") {
				return nil, false
			}
			segs = append(segs, seg)
			s = s[j:]
		case '[':
			if len(s) < 2 || (s[1] != '\'' && s[1] != '"') {
				return nil, false
			}
			q := s[1]
			end := strings.IndexByte(s[2:], q)
			if end < 0 {
				return nil, false
			}
			segs = append(segs, s[2:2+end])
			rest := s[2+end+1:]
			if len(rest) == 0 || rest[0] != ']' {
				return nil, false
			}
			s = rest[1:]
		default:
			return nil, false
		}
	}
	if len(segs) == 0 {
		return nil, false
	}
	return segs, true
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
