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

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/resolve"
)

var _ http.Handler = &IsGrantedValidator{}

// IsGrantedValidator is the /is-granted validating webhook: it allows or denies the use of a granted
// object by availability (which cluster-scoped resources the project may reference).
type IsGrantedValidator struct {
	log     logr.Logger
	cl      client.Client
	mapper  meta.RESTMapper
	factory jsonpath.Factory
}

// NewIsGrantedValidator builds the /is-granted validating webhook.
func NewIsGrantedValidator(log logr.Logger, cl client.Client, mapper meta.RESTMapper, factory jsonpath.Factory) *IsGrantedValidator {
	return &IsGrantedValidator{log: log.WithValues("component", "is-granted"), cl: cl, mapper: mapper, factory: factory}
}

// InstallInto registers the handler on the webhook server.
func (v *IsGrantedValidator) InstallInto(srv webhook.Server) { srv.Register("/is-granted", v) }

func (v *IsGrantedValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	log := v.log.WithValues("namespace", req.Namespace, "name", req.Name, "resource", req.Resource.String())

	resp, err := v.decide(r.Context(), req, log)
	if err != nil {
		log.Error(err, "is-granted decision failed")
		http.Error(w, "is-granted decision failed", http.StatusInternalServerError)
		return
	}
	review.Response = resp
	writeReview(w, review)
}

// decide returns the admission response. Availability is enforced for every matched reference; on
// UPDATE values already present in the old object are grandfathered so existing objects are not broken.
func (v *IsGrantedValidator) decide(ctx context.Context, req *admissionv1.AdmissionRequest, log logr.Logger) (*admissionv1.AdmissionResponse, error) {
	// Never police a system / cluster-component / module writer. Every module's resources land in
	// project namespaces via that module's Helm release applied by the deckhouse-controller (group
	// system:serviceaccounts:d8-system); denying or even stalling such a request fails the install,
	// which addon-operator retries forever, deadlocking the module's queue. The grant allow-list is for
	// PROJECT USERS only. This mirrors (and backstops) the apiserver-level matchConditions on the
	// webhook configuration in case the request still reaches the handler. See protect.go.
	if isSystemRequest(req) {
		return allowedResponse(req.UID), nil
	}
	if namespaces.IsSystem(req.Namespace) || req.SubResource != "" || req.Namespace == "" {
		return allowedResponse(req.UID), nil
	}
	if len(req.Object.Raw) == 0 {
		return allowedResponse(req.UID), nil
	}

	group, version, resourcePlural := req.Resource.Group, req.Resource.Version, req.Resource.Resource

	refs, err := resolve.ReferencesForRequest(ctx, v.cl, group, version, resourcePlural)
	if err != nil {
		return nil, fmt.Errorf("references for request: %w", err)
	}
	if len(refs) == 0 {
		return allowedResponse(req.UID), nil
	}

	ns := &corev1.Namespace{}
	if err := v.cl.Get(ctx, client.ObjectKey{Name: req.Namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return allowedResponse(req.UID), nil
		}
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	project := resolve.ProjectName(ns)

	grants, err := resolve.GrantsForLabels(ctx, v.cl, ns.Labels)
	if err != nil {
		return nil, fmt.Errorf("applicable grants: %w", err)
	}

	obj := map[string]any{}
	if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	var oldObj map[string]any
	if req.Operation == admissionv1.Update && len(req.OldObject.Raw) > 0 {
		oldObj = map[string]any{}
		if err := json.Unmarshal(req.OldObject.Raw, &oldObj); err != nil {
			return nil, fmt.Errorf("decode old object: %w", err)
		}
	}

	// Resolve availability once per definition (several references may share one).
	resolvedByDef := map[string]*resolve.Resolved{}

	for _, mr := range refs {
		fp, ok := engine.SelectFieldPath(mr.Reference.Spec.FieldPaths, group, version)
		if !ok {
			continue
		}
		guardOK, err := engine.EvalMatch(v.factory, fp.Match, obj)
		if err != nil {
			return nil, fmt.Errorf("eval match: %w", err)
		}
		if !guardOK {
			continue
		}
		names, err := engine.StringValuesAt(v.factory, obj, fp.Path)
		if err != nil {
			return nil, fmt.Errorf("read field %q: %w", fp.Path, err)
		}
		if len(names) == 0 {
			continue
		}

		// Grandfather values already present in the old object on UPDATE.
		old := map[string]struct{}{}
		if oldObj != nil {
			if oldVals, err := engine.StringValuesAt(v.factory, oldObj, fp.Path); err == nil {
				for _, n := range oldVals {
					old[n] = struct{}{}
				}
			}
		}

		def := mr.Definition
		resolved := resolvedByDef[def.Name]
		if resolved == nil {
			resolved, err = resolve.Resolve(ctx, v.cl, v.mapper, def, resolve.EntriesFor(grants, def.Name))
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", def.Name, err)
			}
			resolvedByDef[def.Name] = resolved
		}

		for _, name := range names {
			if name == "" {
				continue
			}
			if _, grandfathered := old[name]; grandfathered {
				continue
			}
			if !resolved.Decide(name) {
				msg := fmt.Sprintf(
					"[multitenancy] %s %q references %q which is not available to project %q. "+
						"Ask the cluster administrator to grant it.",
					req.Kind.Kind, req.Name, name, project)
				log.Info("denied: not available", "value", name)
				return deniedResponse(req.UID, msg), nil
			}
		}
	}

	return allowedResponse(req.UID), nil
}
