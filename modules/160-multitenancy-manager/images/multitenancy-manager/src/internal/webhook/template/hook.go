/*
Copyright 2024 Flant JSC

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

package template

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/templategrants"
	"controller/internal/validate"
)

func Register(runtimeManager manager.Manager, serviceAccount string) {
	hook := &webhook.Admission{Handler: &validator{
		client:         runtimeManager.GetClient(),
		reader:         runtimeManager.GetAPIReader(),
		serviceAccount: serviceAccount,
	}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha1/templates", hook)
}

type validator struct {
	serviceAccount string
	client         client.Client
	// reader is the direct (uncached) API reader, used for the grant-policy reference lookups so a
	// cold cache cannot stall the admission request.
	reader client.Reader
}

// Handle validates a ProjectTemplate. The webhook rule matches both v1alpha1 and v1alpha2; with
// matchPolicy: Equivalent the object is delivered in its own version, so unmarshalling into the
// v1alpha2 type (a superset of v1alpha1) covers both — structured fields are simply absent for
// v1alpha1 requests.
func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	template := new(v1alpha2.ProjectTemplate)
	if req.Operation == admissionv1.Create || req.Operation == admissionv1.Update {
		if err := yaml.Unmarshal(req.Object.Raw, template); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// cannot create/update a template with an invalid parameters schema
		schema, err := validate.LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("project template validation failed: %v", err))
		}

		// every structured fromParam reference must point at a parameter declared in parametersSchema,
		// otherwise the field would silently render empty for every project
		for _, ref := range template.Spec.FromParamRefs() {
			if err := validate.ParamPath(schema, ref.Param); err != nil {
				return admission.Denied(fmt.Sprintf("the '%s' project template field '%s' %v", template.Name, ref.Field, err))
			}
		}

		// grantPolicies must reference existing library policies (without a projectSelector)
		if resp := v.validateGrantPolicies(ctx, template); !resp.Allowed {
			return resp
		}

		// the managed ClusterResourceGrantPolicy names this template produces must not collide with
		// the inline slot or with another template's managed names
		if resp := v.validateManagedGrantNames(ctx, template); !resp.Allowed {
			return resp
		}
	}
	if req.Operation == admissionv1.Delete {
		if err := yaml.Unmarshal(req.OldObject.Raw, template); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// cannot delete template if it is used
		projects := new(v1alpha3.ProjectList)
		if err := v.client.List(ctx, projects, client.MatchingLabels{v1alpha3.ResourceLabelTemplate: template.Name}); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if len(projects.Items) > 0 {
			msg := fmt.Sprintf("The '%s' project template cannot be deleted, it is used in the '%s' project", template.Name, projects.Items[0].Name)
			return admission.Denied(msg)
		}
	}
	return admission.Allowed("")
}

// validateGrantPolicies enforces the library convention for spec.grantPolicies: every referenced
// ClusterResourceGrantPolicy must exist and must NOT carry a projectSelector. A policy with a
// projectSelector is already bound to its own set of projects (or is a controller-managed materialized
// policy), so referencing it from a template would double-bind it.
func (v *validator) validateGrantPolicies(ctx context.Context, template *v1alpha2.ProjectTemplate) admission.Response {
	for _, name := range template.Spec.GrantPolicies {
		policy := new(grantsv1alpha1.ClusterResourceGrantPolicy)
		if err := v.reader.Get(ctx, client.ObjectKey{Name: name}, policy); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf(
					"the '%s' project template references ClusterResourceGrantPolicy '%s' which does not exist",
					template.Name, name))
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if policy.Spec.ProjectSelector != nil {
			return admission.Denied(fmt.Sprintf(
				"the '%s' project template references ClusterResourceGrantPolicy '%s' which has a projectSelector; "+
					"grantPolicies may only reference library policies (without a projectSelector)",
				template.Name, name))
		}
	}
	return admission.Allowed("")
}

// validateManagedGrantNames rejects a template whose materialized ClusterResourceGrantPolicy names
// would collide. A managed name is "template-<template>-<source>" where source is "inline" for the
// template's inline resources or the referenced library policy name. Because the parts are joined by
// '-' (which is legal in both names), distinct (template, source) pairs can produce the same name —
// e.g. template "a"+policy "b-c" and template "a-b"+policy "c", or any reference to a policy named
// "inline". Such collisions would otherwise stall the grant materializer on an ownership conflict.
func (v *validator) validateManagedGrantNames(ctx context.Context, template *v1alpha2.ProjectTemplate) admission.Response {
	// "inline" is reserved for the inline-resources slot: a reference to it always maps to the same
	// name as the inline policy, regardless of whether the template currently declares inline resources.
	for _, name := range template.Spec.GrantPolicies {
		if name == templategrants.GrantSourceInline {
			return admission.Denied(fmt.Sprintf(
				"the '%s' project template references a grant policy named '%s', which is reserved for the inline managed policy slot; rename the referenced policy",
				template.Name, name))
		}
	}

	own := templategrants.ManagedNames(template)
	if len(own) == 0 {
		return admission.Allowed("")
	}
	ownNames := make(map[string]struct{}, len(own))
	for _, name := range own {
		ownNames[name] = struct{}{}
	}

	others := new(v1alpha2.ProjectTemplateList)
	if err := v.reader.List(ctx, others); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	for i := range others.Items {
		other := &others.Items[i]
		if other.Name == template.Name {
			continue
		}
		for _, name := range templategrants.ManagedNames(other) {
			if _, clash := ownNames[name]; clash {
				return admission.Denied(fmt.Sprintf(
					"the '%s' project template would produce managed ClusterResourceGrantPolicy '%s', which collides with the '%s' project template; rename the template or the referenced policy",
					template.Name, name, other.Name))
			}
		}
	}
	return admission.Allowed("")
}
