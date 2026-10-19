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

// NodeStaticPodRequestValidator refuses an object name the NodeConfig field would not take or
// that has a writer, and a manifest ValidateStaticPodManifest refuses or whose pod has a writer.
// Two objects naming one pod are settled by the controller, which sees them all.
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

	if err := deckhousev1alpha1.ValidateStaticPodName(nspr.Name); err != nil {
		return admission.Denied(fmt.Sprintf("metadata.name is refused: %s", err))
	}

	if deckhousev1alpha1.IsReservedStaticPodName(nspr.Name) {
		return admission.Denied(fmt.Sprintf(
			"metadata.name %q is reserved for a manifest that the node agent or a bashible step writes", nspr.Name))
	}

	pod, err := deckhousev1alpha1.ValidateStaticPodManifest(nspr.Spec.Manifest)
	if err != nil {
		return admission.Denied(fmt.Sprintf(".spec.manifest is refused: %s", err))
	}
	if deckhousev1alpha1.IsReservedStaticPod(pod) {
		return admission.Denied(fmt.Sprintf(
			".spec.manifest is refused: the node agent or a bashible step already writes a manifest for the pod %s", pod))
	}

	return admission.Allowed("")
}
