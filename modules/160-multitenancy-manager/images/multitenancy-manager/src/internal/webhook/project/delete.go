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

package project

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// RegisterDelete installs the webhook that checks a project delete. It is a webhook of its own (see
// templates/admission/validation.yaml), since a DELETE carries no project to validate and reads what
// the create and update checks do not.
func RegisterDelete(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &deleteValidator{
		client: runtimeManager.GetClient(),
		reader: runtimeManager.GetAPIReader(),
	}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projects/delete", hook)
}

// deleteValidator lets a project go only when the requester may delete each of its namespaces.
// Deleting a project uninstalls its release and deletes its namespaces with every workload in them
// (the project manager's Delete and the ProjectNamespace reconciler), and every namespace belongs to
// a project. A right to delete projects that does not come with the right to delete namespaces would
// otherwise remove any namespace through its project.
type deleteValidator struct {
	// client sends the SubjectAccessReviews.
	client client.Client
	// reader reads the namespaces of the project from the API server. The cache may not have seen a
	// namespace labelled into the project just before the delete, and on a replica that is not the
	// leader it would start the namespace informer inside the request.
	reader client.Reader
}

func (v *deleteValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// The controller and Deckhouse delete projects without the check. Cluster administrators are not
	// privileged here, the same as in the create and update checks.
	if req.UserInfo.Username == rolebindingwebhook.ControllerServiceAccount ||
		req.UserInfo.Username == rolebindingwebhook.DeckhouseServiceAccount {
		return admission.Allowed("")
	}

	namespaces, err := v.projectNamespaces(ctx, req.Name)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, namespace := range namespaces {
		allowed, reason, err := v.canDeleteNamespace(ctx, req.UserInfo, namespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if allowed {
			continue
		}

		message := fmt.Sprintf("user %q may not delete the %q project: deleting it deletes the %q namespace, and the user is not allowed to delete that namespace",
			req.UserInfo.Username, req.Name, namespace)
		if reason != "" {
			message += ": " + reason
		}
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// projectNamespaces returns the namespaces that go with the project, found the way the controller
// finds them: the main namespace by the project name, and the namespaces with the project label.
// A namespace that does not exist is not returned.
func (v *deleteValidator) projectNamespaces(ctx context.Context, project string) ([]string, error) {
	labelled := new(corev1.NamespaceList)
	if err := v.reader.List(ctx, labelled, client.MatchingLabels{v1alpha3.ResourceLabelProject: project}); err != nil {
		return nil, fmt.Errorf("list the namespaces of the '%s' project: %w", project, err)
	}

	names := make([]string, 0, len(labelled.Items)+1)
	for _, namespace := range labelled.Items {
		names = append(names, namespace.Name)
	}

	main := new(corev1.Namespace)
	switch err := v.reader.Get(ctx, client.ObjectKey{Name: project}, main); {
	case err == nil:
		names = append(names, main.Name)
	case !apierrors.IsNotFound(err):
		return nil, fmt.Errorf("get the '%s' namespace: %w", project, err)
	}

	slices.Sort(names)
	return slices.Compact(names), nil
}

// canDeleteNamespace asks the API server whether the requester may delete the namespace. The review
// carries the attributes the API server authorizes a namespace delete with: the namespace is both the
// object and the namespace of the request, so a RoleBinding inside it counts as it does for
// "d8 k delete namespace".
func (v *deleteValidator) canDeleteNamespace(ctx context.Context, user authenticationv1.UserInfo, namespace string) (bool, string, error) {
	extra := make(map[string]authorizationv1.ExtraValue, len(user.Extra))
	for key, values := range user.Extra {
		extra[key] = authorizationv1.ExtraValue(values)
	}

	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   user.Username,
			UID:    user.UID,
			Groups: user.Groups,
			Extra:  extra,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "delete",
				Resource:  "namespaces",
				Name:      namespace,
			},
		},
	}
	if err := v.client.Create(ctx, review); err != nil {
		return false, "", fmt.Errorf("review the delete of the '%s' namespace: %w", namespace, err)
	}

	return review.Status.Allowed, review.Status.Reason, nil
}
