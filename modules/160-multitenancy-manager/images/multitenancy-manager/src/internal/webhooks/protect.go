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

// ProtectValidator is the /protect validating webhook: it keeps the controller-owned
// AvailableClusterResource catalog read-only to everyone but the controller (and system controllers).
type ProtectValidator struct {
	log          logr.Logger
	controllerSA string // e.g. system:serviceaccount:d8-multitenancy-manager:multitenancy-manager
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
	if err := decodeReview(w, r, review); err != nil {
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

// systemBypassUsernames / systemBypassGroups mirror the exemptions Deckhouse uses to protect its own
// heritage objects (see modules/002-deckhouse validation): cluster components and system/module
// controllers must never be blocked by these webhooks. They MUST stay in sync with the apiserver-level
// matchConditions on the webhook configurations (see hooks/configure_grant_*_webhook.go and
// templates/admission/validation.yaml) so the handler-level backstop and the CEL pre-filter agree —
// the CEL conditions match by username AND by group, so this check does too.
var systemBypassUsernames = []string{
	"system:apiserver",
	"system:serviceaccount:d8-system:deckhouse",
	"system:serviceaccount:d8-multitenancy-manager:multitenancy-manager",
}

var systemBypassGroups = []string{
	"system:nodes",
	"system:masters",
	"system:serviceaccounts:kube-system",
	"system:serviceaccounts:d8-system",
}

// isSystemRequest reports whether the request comes from a cluster component or system/module
// controller that must bypass these webhooks. Checks both the username and the groups to match the
// apiserver-level matchConditions exactly.
func isSystemRequest(req *admissionv1.AdmissionRequest) bool {
	for _, u := range systemBypassUsernames {
		if req.UserInfo.Username == u {
			return true
		}
	}
	for _, g := range req.UserInfo.Groups {
		for _, b := range systemBypassGroups {
			if g == b {
				return true
			}
		}
	}
	return false
}

func (p *ProtectValidator) decide(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// The controller may always write its own objects.
	if p.controllerSA != "" && req.UserInfo.Username == p.controllerSA {
		return allowedResponse(req.UID)
	}

	// System controllers and cluster components bypass protection (namespace
	// teardown, garbage collection, kubelet, masters) — mirrors how Deckhouse
	// protects its heritage:deckhouse objects.
	if isSystemRequest(req) {
		return allowedResponse(req.UID)
	}

	switch req.Kind.Kind {
	case "AvailableClusterResource":
		return deniedResponse(req.UID,
			"[multitenancy] AvailableClusterResource is a read-only catalog managed by multitenancy-manager.")
	default:
		return allowedResponse(req.UID)
	}
}
