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

// automatedSystemWriterGroups are the AUTOMATED system writers whose requests must never be denied by
// the grant guardrail, otherwise a module's Helm release (applied by the deckhouse-controller from
// system:serviceaccounts:d8-system) or the reconcile loop of user-authz-controller (which writes
// the AuthorizationRule RoleBindings into project namespaces from
// system:serviceaccounts:d8-user-authz) deadlocks. Unlike protect.go's broader systemBypassGroups,
// system:masters is absent here: the handler itself still polices a cluster-admin (unit tests call
// the handler directly). In-cluster, the matchConditions of the grant webhooks skip system:masters
// before this code runs; they live in hooks/configure_grant_validation_webhook.go
// (systemWriterMatchConditions). The Project/ProjectTemplate/PRB/PN/CPRB webhooks in
// templates/admission/validation.yaml do NOT skip system:masters -- only these grant webhooks do.
var automatedSystemWriterGroups = map[string]struct{}{
	"system:nodes":                         {},
	"system:serviceaccounts:kube-system":   {},
	"system:serviceaccounts:d8-system":     {},
	"system:serviceaccounts:d8-user-authz": {},
}

func isAutomatedSystemWriter(req *admissionv1.AdmissionRequest) bool {
	for _, g := range req.UserInfo.Groups {
		if _, ok := automatedSystemWriterGroups[g]; ok {
			return true
		}
	}
	return false
}

var _ http.Handler = &IsGrantedValidator{}

// IsGrantedValidator is the /is-granted validating webhook: it allows or denies the use of a granted
// object by availability (which cluster-scoped resources the project may reference).
type IsGrantedValidator struct {
	log     logr.Logger
	cl      client.Reader
	mapper  meta.RESTMapper
	factory jsonpath.Factory
}

// NewIsGrantedValidator builds the /is-granted validating webhook. cl is a direct (uncached) API reader
// on purpose: the cache-backed client lazily starts an informer on first read of a type and blocks on
// its sync inside the admission request; on a large cluster that first sync can exceed the webhook
// deadline, and the per-request cancellation prevents it from ever completing — every call then times
// out and (with failurePolicy: Fail) piles up retries into a queue lock. A direct reader never depends
// on informer sync, so reads are bounded and the webhook cannot hang.
func NewIsGrantedValidator(log logr.Logger, cl client.Reader, mapper meta.RESTMapper, factory jsonpath.Factory) *IsGrantedValidator {
	return &IsGrantedValidator{log: log.WithValues("component", "is-granted"), cl: cl, mapper: mapper, factory: factory}
}

// InstallInto registers the handler on the webhook server.
func (v *IsGrantedValidator) InstallInto(srv webhook.Server) { srv.Register("/is-granted", v) }

func (v *IsGrantedValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	log := v.log.WithValues("namespace", req.Namespace, "name", req.Name, "resource", req.Resource.String())

	// Hard-bound the decision so the webhook always answers quickly and can never become a queue lock.
	ctx, cancel := context.WithTimeout(r.Context(), webhookDecisionTimeout)
	defer cancel()

	resp, err := v.decide(ctx, req, log)
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
	// NEVER deny an AUTOMATED system writer. The deckhouse-controller SA (group
	// system:serviceaccounts:d8-system) applies EVERY module's Helm release server-side; a denial here
	// fails that install, which addon-operator then retries forever — deadlocking the module's queue
	// (this is exactly how an AuthorizationRule/CAR-derived RoleBinding locked the user-authz module).
	// kube-system controllers / the kubelet must not be blocked during teardown either. The grant
	// allow-list exists to police USERS, who instead get a fast, terminal admission denial.
	// NOTE: system:masters is not in automatedSystemWriterGroups, so a direct handler call still
	// polices a cluster-admin. In-cluster, the grant webhooks' matchConditions already skip
	// system:masters — those requests never reach this handler.
	if isAutomatedSystemWriter(req) {
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

	grants, err := resolve.GrantsForNamespace(ctx, v.cl, ns)
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

	// Resolve availability once per definition (several references may share one). unresolvable holds
	// the definitions skipped for a configuration error, so each is logged once per request.
	resolvedByDef := map[string]*resolve.Resolved{}
	unresolvable := map[string]struct{}{}

	for _, mr := range refs {
		idx := engine.SelectFieldPathIndex(mr.Reference.Spec.FieldPaths, group, version, resourcePlural)
		if idx < 0 {
			continue
		}
		fp := mr.Reference.Spec.FieldPaths[idx]
		// A path of one reference that cannot be evaluated skips that reference only, deliberately
		// failing open. The GrantableClusterResourceReference webhook rejects such a path, but it runs
		// with failurePolicy: Ignore, so a broken reference can still be stored (the webhook was down,
		// or the object predates it). Failing the request here instead would, under this webhook's
		// failurePolicy: Fail, block every CREATE/UPDATE of the reference's rule in every project
		// because of one bad object; skipping costs only the checks that reference could not make
		// anyway. The other references are still enforced, and the breakage stays visible in this log
		// and in the reference's FieldPathsValid=False condition.
		guardOK, err := engine.EvalMatch(v.factory, fp.Match, obj)
		if err != nil {
			log.Error(err, "skipping reference: match.fieldPath cannot be evaluated",
				"reference", mr.Reference.Name, "fieldPathIndex", idx, "path", fp.Match.FieldPath)
			continue
		}
		if !guardOK {
			continue
		}
		names, err := engine.StringValuesAt(v.factory, obj, fp.Path)
		if err != nil {
			log.Error(err, "skipping reference: path cannot be evaluated",
				"reference", mr.Reference.Name, "fieldPathIndex", idx, "path", fp.Path)
			continue
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
		if _, skip := unresolvable[def.Name]; skip {
			continue
		}
		resolved := resolvedByDef[def.Name]
		if resolved == nil {
			resolved, err = resolve.Resolve(ctx, v.cl, v.mapper, def, resolve.EntriesFor(grants, def.Name))
			if resolve.IsConfigurationError(err) {
				// A definition that cannot be resolved for a configuration reason (a namespaced
				// grantedResource, or a kind the apiserver does not serve) makes its references inert,
				// deliberately, by the same rule as a broken path above. Retrying cannot fix it, and
				// failing the request under failurePolicy: Fail would block every CREATE/UPDATE of the
				// reference's rule in every project because of one bad registration. The other
				// references are still enforced, and the breakage stays visible in this log and in the
				// definition's GrantedResourceValid condition. Any other Resolve error still fails the
				// request below.
				log.Error(err, "skipping reference: definition cannot be resolved",
					"definition", def.Name, "reference", mr.Reference.Name)
				unresolvable[def.Name] = struct{}{}
				continue
			}
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
