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

// Package projectnamespace validates ProjectNamespace objects: they may be created only in a
// project's main namespace, the resulting "<project>-<name>" namespace must be RFC1123 and within
// the 63-character limit, and must not collide with a namespace owned by another project.
package projectnamespace

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
)

// maxNamespaceNameLength is the Kubernetes limit on a namespace (RFC1123 label) name.
const maxNamespaceNameLength = 63

// Register installs the ProjectNamespace validating webhook.
func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projectnamespaces", hook)
}

type validator struct {
	client client.Client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation == admissionv1.Delete {
		return admission.Allowed("")
	}

	pns := new(v1alpha3.ProjectNamespace)
	if err := yaml.Unmarshal(req.Object.Raw, pns); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// The object must live in the main namespace of an existing, non-virtual project. The main
	// namespace equals the project name, so a ProjectNamespace created in an additional namespace
	// (whose name never matches a Project name) is rejected here too - no recursion.
	if req.Namespace == "default" || req.Namespace == "deckhouse" {
		return admission.Denied("ProjectNamespace cannot be created in a virtual project namespace")
	}
	project := new(v1alpha3.Project)
	if err := v.client.Get(ctx, client.ObjectKey{Name: req.Namespace}, project); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Denied(fmt.Sprintf("namespace %q is not the main namespace of a project; ProjectNamespace may only be created in a project's main namespace", req.Namespace))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if project.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" {
		return admission.Denied("ProjectNamespace cannot be created in a virtual project namespace")
	}

	// Defense in depth: the CRD CEL rule already enforces this, but keep the webhook authoritative.
	resulting := req.Namespace + "-" + pns.Spec.Name
	if len(resulting) > maxNamespaceNameLength {
		return admission.Denied(fmt.Sprintf("the resulting namespace name %q is %d characters long, which exceeds the %d-character limit", resulting, len(resulting), maxNamespaceNameLength))
	}

	// The resulting namespace must not already exist unless it is already owned by this project
	// (idempotent re-create of the same claim).
	existing := new(corev1.Namespace)
	switch err := v.client.Get(ctx, client.ObjectKey{Name: resulting}, existing); {
	case err == nil:
		if existing.Labels[v1alpha3.ResourceLabelProject] != req.Namespace {
			return admission.Denied(fmt.Sprintf("namespace %q already exists and is not owned by project %q", resulting, req.Namespace))
		}
	case !apierrors.IsNotFound(err):
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// spec.features is validated to be a subset of the project features. The Project resource does
	// not model features in this codebase, so this is a no-op placeholder until project features
	// exist (Card 12 / ADR-2); spec.features is carried through as-is.

	return admission.Allowed("")
}
