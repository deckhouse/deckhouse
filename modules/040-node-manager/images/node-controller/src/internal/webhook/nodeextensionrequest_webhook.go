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

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

var nerWebhookLog = logf.Log.WithName("nodeextensionrequest-webhook")

// NodeExtensionRequestValidator refuses a sysext name reserved for a platform
// extension, which the CRD cannot express. Name, digest and kernel-module
// clashes between requests are left to the nodeconfig controller: it settles
// them cluster-wide and writes the reason onto the request that lost.
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

	if name := ner.Spec.Sysext.Name; deckhousev1alpha1.IsReservedSysextName(name) {
		return admission.Denied(fmt.Sprintf(
			"it is forbidden to set .spec.sysext.name to %q: the name is reserved for a platform extension", name))
	}

	return admission.Allowed("")
}
