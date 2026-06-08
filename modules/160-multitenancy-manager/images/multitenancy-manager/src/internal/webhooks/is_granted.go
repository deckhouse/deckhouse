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

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/quota"
	"controller/internal/resolve"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ http.Handler = &IsGrantedValidator{}

// IsGrantedValidator is the /is-granted validating webhook: it allows or denies the use of a granted
// object (by availability) and enforces object quota.
type IsGrantedValidator struct {
	log     logr.Logger
	cl      client.Client
	factory jsonpath.Factory
}

// NewIsGrantedValidator builds the /is-granted validating webhook.
func NewIsGrantedValidator(log logr.Logger, cl client.Client, factory jsonpath.Factory) *IsGrantedValidator {
	return &IsGrantedValidator{log: log.WithValues("component", "is-granted"), cl: cl, factory: factory}
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

// decide returns the admission response. It fails open only on its own internal errors (returned as
// error), never silently — availability for opt-in (None) / excluded resources is enforced even when
// no grant matches the project.
func (v *IsGrantedValidator) decide(ctx context.Context, req *admissionv1.AdmissionRequest, log logr.Logger) (*admissionv1.AdmissionResponse, error) {
	// System namespaces and subresources are out of scope.
	if namespaces.IsSystem(req.Namespace) || req.SubResource != "" || req.Namespace == "" {
		return allowedResponse(req.UID), nil
	}
	if len(req.Object.Raw) == 0 {
		return allowedResponse(req.UID), nil
	}

	group, version, resourcePlural := req.Resource.Group, req.Resource.Version, req.Resource.Resource

	regs, err := resolve.RegistrationsForRequest(ctx, v.cl, group, version, resourcePlural)
	if err != nil {
		return nil, fmt.Errorf("registrations for request: %w", err)
	}
	if len(regs) == 0 {
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

	var pool *v1alpha1.GrantQuota
	var projectNamespaces []string

	for _, reg := range regs {
		entries := resolve.EntriesFor(grants, reg.Name)
		resolved, err := resolve.Resolve(ctx, v.cl, reg, entries)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", reg.Name, err)
		}

		contribs, err := engine.Contributions(v.factory, reg, obj, group, version, resourcePlural)
		if err != nil {
			return nil, fmt.Errorf("contributions: %w", err)
		}
		if len(contribs) == 0 {
			continue
		}

		// Diff-based UPDATE: values already present in the old object are grandfathered.
		oldNames := map[string]struct{}{}
		if oldObj != nil {
			oldContribs, err := engine.Contributions(v.factory, reg, oldObj, group, version, resourcePlural)
			if err != nil {
				return nil, fmt.Errorf("old contributions: %w", err)
			}
			for _, c := range oldContribs {
				oldNames[c.Name] = struct{}{}
			}
		}

		// Availability check.
		for _, c := range contribs {
			if _, grandfathered := oldNames[c.Name]; grandfathered {
				continue
			}
			if !resolved.Decide(c.Name) {
				msg := fmt.Sprintf(
					"[multitenancy] %s %q references %q which is not available to project %q. "+
						"Ask the cluster administrator to grant it.",
					req.Kind.Kind, req.Name, c.Name, project)
				log.Info("denied: not available", "value", c.Name)
				return deniedResponse(req.UID, msg), nil
			}
		}

		// Quota check.
		if pool == nil {
			pool, err = quota.Pool(ctx, v.cl, project)
			if err != nil {
				return nil, fmt.Errorf("pool: %w", err)
			}
		}
		if pool == nil {
			continue
		}
		if projectNamespaces == nil {
			projectNamespaces, err = resolve.ProjectNamespaces(ctx, v.cl, project)
			if err != nil {
				return nil, fmt.Errorf("project namespaces: %w", err)
			}
		}
		gvk := schema.GroupVersionKind{Group: req.Kind.Group, Version: req.Kind.Version, Kind: req.Kind.Kind}
		used, err := quota.ProjectUsage(ctx, v.cl, v.factory, reg, gvk, resourcePlural, projectNamespaces, req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("project usage: %w", err)
		}
		adding := quota.ContributionUsage(contribs)
		if viol := quota.Check(pool, reg.Name, used, adding); viol != nil {
			msg := fmt.Sprintf(
				"[multitenancy] %s %q exceeds the object quota of project %q (%s).",
				req.Kind.Kind, req.Name, project, viol.String())
			log.Info("denied: quota", "violation", viol.String())
			return deniedResponse(req.UID, msg), nil
		}
	}

	return allowedResponse(req.UID), nil
}
