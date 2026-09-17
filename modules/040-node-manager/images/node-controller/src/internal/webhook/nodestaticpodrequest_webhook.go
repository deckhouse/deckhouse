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

var nsprWebhookLog = logf.Log.WithName("nodestaticpodrequest-webhook")

// NodeStaticPodRequestValidator refuses what the CRD cannot express: an object
// name reserved for a control-plane manifest, and a manifest that is not a valid
// Pod with a name and a namespace — the same single question the node's loader
// asks, answered by the same decoder. Refused here because a node that refuses a
// manifest refuses the whole NodeConfig with it — one typo would stop that node
// converging on anything at all. The nodeconfig controller checks the same two
// things again and writes the reason onto the object, for the documents that
// were already in the cluster when this webhook arrived.
//
// Two objects whose manifests name one pod are NOT refused here. Seeing that
// needs a live listing of every other object, and the loser needs a status to be
// told in — so the controller settles it cluster-wide and writes Conflict onto
// the younger one, the same split NodeExtensionRequest makes.
type NodeStaticPodRequestValidator struct {
	decoder admission.Decoder
}

// Handle validates a NodeStaticPodRequest on CREATE and UPDATE.
func (w *NodeStaticPodRequestValidator) Handle(_ context.Context, req admission.Request) admission.Response {
	nsprWebhookLog.Info("validating nodestaticpodrequest", "name", req.Name, "operation", req.Operation)

	nspr := &deckhousev1alpha1.NodeStaticPodRequest{}
	if err := w.decoder.Decode(req, nspr); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if deckhousev1alpha1.IsReservedStaticPodName(nspr.Name) {
		return admission.Denied(fmt.Sprintf(
			"it is forbidden to name a NodeStaticPodRequest %q: the name belongs to a control-plane manifest the node agent writes itself", nspr.Name))
	}

	if _, err := deckhousev1alpha1.ValidateStaticPodManifest(nspr.Spec.Manifest); err != nil {
		return admission.Denied(fmt.Sprintf(".spec.manifest is refused: %s", err))
	}

	return admission.Allowed("")
}
