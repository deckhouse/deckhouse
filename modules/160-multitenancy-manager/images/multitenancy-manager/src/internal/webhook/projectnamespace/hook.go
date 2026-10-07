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
// the 63-character limit, and must not collide with a namespace owned by another project, with a
// project of that name, with the claim of another ProjectNamespace or with the ServiceAccount
// subjects that bindings of other projects still have for that name.
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
	"controller/internal/rolebinding"
)

// maxNamespaceNameLength is the Kubernetes limit on a namespace (RFC1123 label) name.
const maxNamespaceNameLength = 63

// Register installs the ProjectNamespace validating webhook.
func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient(), reader: runtimeManager.GetAPIReader()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projectnamespaces", hook)
}

type validator struct {
	client client.Client
	// reader is the direct API reader for every object the webhook reads. These are the project the
	// ProjectNamespace is created in, the namespace that already has the resulting name, and what can
	// take that name, which is a project with it, the ProjectNamespaces that claim it and the role
	// bindings that still name its ServiceAccounts. A GitOps sync applies such an object and this
	// ProjectNamespace one after the other, and the cache may not have seen the first one yet. Through
	// the cache, a ProjectNamespace applied right after its Project would be refused as not being in a
	// project. The cache would also start the informers of the bindings inside the request on a replica
	// that is not the leader.
	reader client.Reader
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation == admissionv1.Delete {
		return admission.Allowed("")
	}

	pns := new(v1alpha3.ProjectNamespace)
	if err := yaml.Unmarshal(req.Object.Raw, pns); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// A ProjectNamespace that is being deleted waits only for the controller to remove its finalizer,
	// which is an update, and it creates no namespace any more. The checks below would refuse that
	// update while a namespace of someone else has the resulting name, which the controller leaves
	// alone, and the object would never go away, nor would the main namespace it lives in.
	if !pns.DeletionTimestamp.IsZero() {
		return admission.Allowed("")
	}

	// The object must live in the main namespace of an existing, non-virtual project. The main
	// namespace equals the project name, so a ProjectNamespace created in an additional namespace
	// (whose name never matches a Project name) is rejected here too - no recursion.
	//
	// This is a *placement* check, so it runs on CREATE alone (mirroring the ProjectRoleBinding
	// webhook). Enforcing it on UPDATE would deadlock project teardown: the controller removes the
	// ProjectNamespace finalizer with an Update, and by then the owning project (and its main
	// namespace) is terminating or already gone, so the lookup below would deny the finalizer
	// removal and the additional namespace could never be cleaned up. An UPDATE can never relocate
	// a ProjectNamespace, and spec.name is immutable (enforced by the CRD), so the resulting-name
	// length and collision invariants below stay correct on UPDATE without the placement gate.
	if req.Operation == admissionv1.Create {
		if req.Namespace == "default" || req.Namespace == "deckhouse" {
			return admission.Denied("ProjectNamespace cannot be created in a virtual project namespace")
		}
		project := new(v1alpha3.Project)
		if err := v.reader.Get(ctx, client.ObjectKey{Name: req.Namespace}, project); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf("namespace %q is not the main namespace of a project; ProjectNamespace may only be created in a project's main namespace", req.Namespace))
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if project.IsVirtual() {
			return admission.Denied("ProjectNamespace cannot be created in a virtual project namespace")
		}
	}

	// Defense in depth: the CRD CEL rule already enforces this, but keep the webhook authoritative.
	resulting := req.Namespace + "-" + pns.Spec.Name
	if len(resulting) > maxNamespaceNameLength {
		return admission.Denied(fmt.Sprintf("the resulting namespace name %q is %d characters long, which exceeds the %d-character limit", resulting, len(resulting), maxNamespaceNameLength))
	}

	// The resulting namespace must not already exist unless it is already owned by this project
	// (idempotent re-create of the same claim).
	existing := new(corev1.Namespace)
	ownsNamespace := false
	switch err := v.reader.Get(ctx, client.ObjectKey{Name: resulting}, existing); {
	case err == nil:
		if existing.Labels[v1alpha3.ResourceLabelProject] != req.Namespace {
			return admission.Denied(fmt.Sprintf("namespace %q already exists and is not owned by project %q", resulting, req.Namespace))
		}
		ownsNamespace = existing.DeletionTimestamp.IsZero()
	case !apierrors.IsNotFound(err):
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// The name must not be taken by anything that has no namespace yet. Both checks run on CREATE
	// alone: spec.name is immutable, so a name that was free when the object was created stays its
	// own, and checking it again on UPDATE could only refuse the finalizer removal that tears the
	// object down.
	if req.Operation == admissionv1.Create {
		denied, err := v.nameTaken(ctx, resulting, req.Namespace, ownsNamespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if denied != "" {
			return admission.Denied(denied)
		}
	}

	// spec.features is validated to be a subset of the project features. The Project resource does
	// not model features in this codebase, so this is a no-op placeholder until project features
	// exist; spec.features is carried through as-is.

	return admission.Allowed("")
}

// nameTaken returns a denial message when the resulting namespace name of the project is taken by a
// project, by another ProjectNamespace whose namespace does not exist yet or by the ServiceAccount
// subjects of a binding of another project, and an empty string when it is free. ownsNamespace says
// that the namespace exists, belongs to the project and is not being deleted: then no name changes
// hands, and the ServiceAccount subjects of other bindings are no reason to refuse.
func (v *validator) nameTaken(ctx context.Context, resulting, project string, ownsNamespace bool) (string, error) {
	// A project makes its namespace under its own name only when the controller gets to it, so the
	// project alone holds the name until then.
	switch err := v.reader.Get(ctx, client.ObjectKey{Name: resulting}, new(v1alpha3.Project)); {
	case err == nil:
		return fmt.Sprintf("the resulting namespace name %q is the name of a project", resulting), nil
	case !apierrors.IsNotFound(err):
		return "", fmt.Errorf("get the %q project: %w", resulting, err)
	}

	// Two projects can arrive at one name: "team" with "a-backend" and "team-a" with "backend" both
	// claim "team-a-backend". The object being created is not stored yet, so any claim found is
	// another one.
	claim, err := Claiming(ctx, v.reader, resulting)
	if err != nil {
		return "", err
	}
	if claim != nil {
		return fmt.Sprintf("the resulting namespace %q is already claimed by the %q ProjectNamespace of project %q",
			resulting, claim.Name, claim.Namespace), nil
	}

	// A binding of another project, or a cluster-wide one, can still name the ServiceAccounts of a
	// namespace under this name that is gone, and the new namespace would hand them to this project.
	// A binding of this project names them to bring them back, which is what recreating a deleted
	// ProjectNamespace is for.
	if ownsNamespace {
		return "", nil
	}
	reason, err := rolebinding.ServiceAccountGrantConflict(ctx, v.reader, resulting, project)
	if err != nil {
		return "", err
	}
	if reason != "" {
		return fmt.Sprintf("the resulting namespace %q cannot be created, %s", resulting, reason), nil
	}
	return "", nil
}

// Claiming returns the ProjectNamespace that claims the namespace name, or nil when none does. A
// ProjectNamespace in the main namespace of project P claims "P-<spec.name>" from the moment it is
// created, before the controller creates that namespace. Any dash of the name can separate P from
// the suffix, so the ProjectNamespaces of every such P are read.
func Claiming(ctx context.Context, reader client.Reader, name string) (*v1alpha3.ProjectNamespace, error) {
	for i := 1; i < len(name)-1; i++ {
		if name[i] != '-' {
			continue
		}
		project, suffix := name[:i], name[i+1:]
		list := new(v1alpha3.ProjectNamespaceList)
		if err := reader.List(ctx, list, client.InNamespace(project)); err != nil {
			return nil, fmt.Errorf("list the ProjectNamespaces of the %q project: %w", project, err)
		}
		for j := range list.Items {
			if list.Items[j].Spec.Name == suffix {
				return &list.Items[j], nil
			}
		}
	}
	return nil, nil
}
