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

package webhook

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

var nerWebhookLog = logf.Log.WithName("nodeextensionrequest-webhook")

// NodeExtensionRequestValidator refuses a reserved extension name and a change of kind, which
// the CRD cannot express, and a missing or unknown kind. Name, digest and kernel-module clashes
// are the nodeconfig controller's: it settles them cluster-wide and reports them on the loser.
type NodeExtensionRequestValidator struct {
	decoder admission.Decoder
}

// Handle validates a NodeExtensionRequest on CREATE and UPDATE.
func (w *NodeExtensionRequestValidator) Handle(_ context.Context, req admission.Request) admission.Response {
	nerWebhookLog.Info("validating nodeextensionrequest", "name", req.Name, "operation", req.Operation)

	ner := &deckhousev1alpha1.NodeExtensionRequest{}
	if err := w.decoder.Decode(req, ner); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if name := ner.Spec.Extension.Name; deckhousev1alpha1.IsReservedExtensionName(name) {
		return admission.Denied(fmt.Sprintf(
			"it is forbidden to set .spec.extension.name to %q: the name is reserved for a platform extension", name))
	}

	kind := ner.Spec.Extension.Kind
	if kind != deckhousev1alpha1.ExtensionKindPackage && kind != deckhousev1alpha1.ExtensionKindSystemExtension {
		return admission.Denied(fmt.Sprintf(".spec.extension.kind must be %s or %s, not %q",
			deckhousev1alpha1.ExtensionKindPackage, deckhousev1alpha1.ExtensionKindSystemExtension, kind))
	}

	if req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}
	old := &deckhousev1alpha1.NodeExtensionRequest{}
	if err := w.decoder.DecodeRaw(req.OldObject, old); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if old.Spec.Extension.Kind != kind {
		return admission.Denied(fmt.Sprintf(
			".spec.extension.kind cannot be changed from %q to %q: delete the NodeExtensionRequest and create it again",
			old.Spec.Extension.Kind, kind))
	}

	return admission.Allowed("")
}
