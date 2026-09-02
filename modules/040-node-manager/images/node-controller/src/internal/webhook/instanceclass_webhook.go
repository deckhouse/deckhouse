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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// InstanceClassDeleteValidator denies deletion of a provider InstanceClass that is
// still referenced by NodeGroups (reimplementation of the shell hook
// modules/040-node-manager/webhooks/validating/instance_class.py).
type InstanceClassDeleteValidator struct{}

func (w *InstanceClassDeleteValidator) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Delete {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown operation %s", req.Operation))
	}

	if len(req.OldObject.Raw) == 0 {
		return admission.Allowed("")
	}

	var oldObject struct {
		Status struct {
			NodeGroupConsumers []string `json:"nodeGroupConsumers"`
		} `json:"status"`
	}
	if err := json.Unmarshal(req.OldObject.Raw, &oldObject); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("parse oldObject: %w", err))
	}

	if len(oldObject.Status.NodeGroupConsumers) > 0 {
		return admission.Denied(fmt.Sprintf(
			"%s/%s cannot be deleted because it is being used by NodeGroup: %s",
			req.Kind.Kind, req.Name, strings.Join(oldObject.Status.NodeGroupConsumers, ", ")))
	}

	return admission.Allowed("")
}
