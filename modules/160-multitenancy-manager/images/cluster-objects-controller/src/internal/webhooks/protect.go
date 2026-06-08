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
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ http.Handler = &ProtectValidator{}

// ProtectValidator is the /protect validating webhook: it keeps the controller-owned status surfaces
// (AvailableResource catalog and GrantQuota usage) read-only to everyone but the controller. The
// GrantQuota pool spec itself is governed by RBAC (cluster-admin only), so spec writes pass here.
type ProtectValidator struct {
	log          logr.Logger
	controllerSA string // e.g. system:serviceaccount:d8-multitenancy-manager:cluster-objects-controller
}

// NewProtectValidator builds the /protect validating webhook. controllerSA is the username of the
// controller's service account, which is always allowed.
func NewProtectValidator(log logr.Logger, controllerSA string) *ProtectValidator {
	return &ProtectValidator{log: log.WithValues("component", "protect"), controllerSA: controllerSA}
}

// InstallInto registers the handler on the webhook server.
func (p *ProtectValidator) InstallInto(srv webhook.Server) { srv.Register("/protect", p) }

func (p *ProtectValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := &admissionv1.AdmissionReview{}
	if err := decodeReview(r, review); err != nil {
		http.Error(w, "invalid AdmissionReview: "+err.Error(), http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		http.Error(w, "AdmissionReview without request", http.StatusBadRequest)
		return
	}
	review.Response = p.decide(review.Request)
	writeReview(w, review)
}

func (p *ProtectValidator) decide(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// The controller may always write its own objects.
	if p.controllerSA != "" && req.UserInfo.Username == p.controllerSA {
		return allowedResponse(req.UID)
	}

	switch req.Kind.Kind {
	case "AvailableResource":
		return deniedResponse(req.UID,
			"[multitenancy] AvailableResource is a read-only catalog managed by multitenancy-manager.")
	case "GrantQuota":
		// Usage status is controller-owned; only the controller may write it.
		if req.SubResource == "status" {
			return deniedResponse(req.UID,
				"[multitenancy] GrantQuota status is managed by multitenancy-manager and is read-only.")
		}
		// Spec writes (the pool) are governed by RBAC — cluster admins only; let them through.
		return allowedResponse(req.UID)
	default:
		return allowedResponse(req.UID)
	}
}
