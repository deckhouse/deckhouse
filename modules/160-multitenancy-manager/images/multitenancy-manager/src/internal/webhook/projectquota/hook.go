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

// Package projectquota validates the keys of a ProjectQuota and warns when a change of the limits puts
// the project above its quota. The status subresource is not matched: only the controller writes it.
package projectquota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/internal/quota"
)

// Path is the path the webhook serves.
const Path = "/validate/v1alpha1/projectquotas"

// Register installs the ProjectQuota validating webhook.
func Register(runtimeManager manager.Manager, keys quota.Discovery) {
	runtimeManager.GetWebhookServer().Register(Path, &webhook.Admission{Handler: &validator{keys: keys}})
}

type validator struct {
	// keys looks up the resources the count/ keys name. Optional.
	keys quota.Discovery
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}
	pq := new(v1alpha1.ProjectQuota)
	if err := json.Unmarshal(req.Object.Raw, pq); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	old := new(v1alpha1.ProjectQuota)
	if req.Operation == admissionv1.Update {
		if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	check, err := quota.CheckKeys(pq.Spec.Hard, old.Spec.Hard, v.keys)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if len(check.Denied) > 0 {
		return admission.Denied(fmt.Sprintf("The '%s' ProjectQuota is invalid: %s", pq.Name, strings.Join(check.Denied, "; ")))
	}
	return admission.Allowed("").WithWarnings(append(check.Warnings, belowUsage(pq, old)...)...)
}

// belowUsage warns of the limits a change sets below what the project uses already. Such a change is
// allowed: nothing running is evicted, and creating more of the resource is denied until the usage is
// back within the limit.
func belowUsage(pq, old *v1alpha1.ProjectQuota) []string {
	hard := quota.Canonical(pq.Spec.Hard)
	previous := quota.Canonical(old.Spec.Hard)
	var warnings []string
	for key, limit := range hard {
		used, ok := old.Status.Used[key]
		if !ok || used.Cmp(limit) <= 0 {
			continue
		}
		if before, ok := previous[key]; ok && before.Cmp(limit) == 0 {
			continue
		}
		warnings = append(warnings, belowUsageWarning(key, limit, used))
	}
	slices.Sort(warnings)
	return warnings
}

func belowUsageWarning(key corev1.ResourceName, limit, used resource.Quantity) string {
	return fmt.Sprintf("%s: the limit %s is below what the project uses, %s; nothing running is evicted, and creating more is denied until the usage is back within the limit",
		key, limit.String(), used.String())
}
