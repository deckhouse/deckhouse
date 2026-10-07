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
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/templategrants"
	"controller/internal/naming"
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

// Handle validates a ProjectTemplate. The admission rule lists v1alpha2 only; matchPolicy:
// Equivalent up-converts a request through the served v1alpha1 to v1alpha2 before delivery, and
// RequestKind still names v1alpha1 (see v1alpha1Erasure). The handler path is registered as
// /validate/v1alpha1/templates for historical reasons.
func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var warnings []string
	template := new(v1alpha2.ProjectTemplate)
	if req.Operation == admissionv1.Create || req.Operation == admissionv1.Update {
		if err := yaml.Unmarshal(req.Object.Raw, template); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if resp := v.v1alpha1Erasure(ctx, req, template); !resp.Allowed {
			return resp
		}

		if resp := newResourcesTemplate(req, template); !resp.Allowed {
			return resp
		}

		// A template renders from its Helm text or from its structured fields and manifests, never from
		// both: the renderer takes the text and leaves the fields unused, which nothing would tell the
		// author.
		if fields := template.Spec.ObjectFields(); template.Spec.HasResourcesTemplate() && len(fields) > 0 {
			return admission.Denied(fmt.Sprintf(
				"the '%s' project template sets resourcesTemplate together with fields that render objects: %s; "+
					"a template renders from one of the two, so remove the other",
				template.Name, strings.Join(fields, ", ")))
		}

		if resp := v.resourcesTemplateRemoval(ctx, req, template); !resp.Allowed {
			return resp
		}

		// cannot create/update a template with an invalid parameters schema
		schema, err := validate.LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("project template validation failed: %v", err))
		}

		// every structured fromParam reference must point at a parameter declared in parametersSchema
		// AND be type-compatible with the field, otherwise the field would silently render empty (or
		// fail to render) for every project on this template
		for _, ref := range template.Spec.FromParamRefs() {
			if err := validate.ParamPath(schema, ref.Param, ref.Type); err != nil {
				return admission.Denied(fmt.Sprintf("the '%s' project template field '%s' %v", template.Name, ref.Field, err))
			}
		}

		// a literal in a field with a fixed set of values must be one of them: a typo used to be
		// accepted, rendered into a label nothing reads or into no NetworkPolicy at all, and the
		// project reported Deployed
		if err := validateLiterals(template); err != nil {
			return admission.Denied(fmt.Sprintf("the '%s' project template %v", template.Name, err))
		}

		// namespaceMetadata.labels cannot carry a label the module owns: the renderer drops it, and the
		// namespace would lack what the template says without a word. A {fromParam} reference is
		// resolved per project and checked by the Project webhook. A template written before this
		// check may already carry such a label; refusing every later edit of it for that label would
		// lock the template, so an update is refused only for a label it sets anew, and the ones it
		// keeps are named in a warning.
		if template.Spec.NamespaceMetadata != nil {
			if labels, isLiteral := template.Spec.NamespaceMetadata.Labels.Literal(); isLiteral {
				previous, err := previousNamespaceLabels(req)
				if err != nil {
					return admission.Errored(http.StatusBadRequest, err)
				}
				set, kept := naming.SplitModuleOwnedLabels(labels, previous)
				if len(set) > 0 {
					return admission.Denied(fmt.Sprintf("the '%s' project template field 'namespaceMetadata.labels' sets labels the module owns: %s",
						template.Name, strings.Join(naming.ModuleOwnedLabelsIn(set), "; ")))
				}
				if len(kept) > 0 {
					warnings = append(warnings, fmt.Sprintf(
						"the '%s' project template field 'namespaceMetadata.labels' sets labels the module owns, and its namespaces do not get them: %s; remove them from the field",
						template.Name, strings.Join(naming.ModuleOwnedLabelsIn(kept), "; ")))
				}
			}
		}

		// the inline grant entries must satisfy the same selector rules the ClusterResourceGrantPolicy
		// schema enforces, or the managed policy this template materializes is refused at admission
		// and the reconciler retries it forever
		if resp := validateInlineGrantSelectors(template); !resp.Allowed {
			return resp
		}

		// grantPolicies must reference existing library policies (without a selector)
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
		user, err := v.projectUsing(ctx, template.Name)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if user != "" {
			msg := fmt.Sprintf("The '%s' project template cannot be deleted, it is used in the '%s' project", template.Name, user)
			return admission.Denied(msg)
		}

		// Nor while a namespace is still rendered from it. The policies of the template select
		// namespaces by that label, so a project that switched to a template that has not rendered yet
		// keeps the rules of this one only while this one exists.
		namespaces := new(corev1.NamespaceList)
		if err := v.client.List(ctx, namespaces, client.MatchingLabels{
			v1alpha3.ResourceLabelTemplate: template.Name,
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		}); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		// A namespace on its way out is rendered from nothing any more: nothing can be created in it.
		namespaces.Items = slices.DeleteFunc(namespaces.Items, func(ns corev1.Namespace) bool { return ns.DeletionTimestamp != nil })
		if len(namespaces.Items) > 0 {
			ns := slices.MinFunc(namespaces.Items, func(a, b corev1.Namespace) int { return strings.Compare(a.Name, b.Name) })
			project := ns.Labels[v1alpha3.ResourceLabelProject]
			hint := "wait until the project renders its new template"
			switch err := v.client.Get(ctx, client.ObjectKey{Name: project}, new(v1alpha3.Project)); {
			case apierrors.IsNotFound(err):
				hint = "the project is gone; delete the namespace first"
			case err != nil:
				return admission.Errored(http.StatusInternalServerError, err)
			}
			msg := fmt.Sprintf("The '%s' project template cannot be deleted, the '%s' namespace of the '%s' project is still rendered from it; %s",
				template.Name, ns.Name, project, hint)
			return admission.Denied(msg)
		}
	}
	return admission.Allowed("").WithWarnings(warnings...)
}

// v1alpha1Erasure refuses an update through deckhouse.io/v1alpha1 of a template that has fields
// v1alpha1 cannot describe. Such a request carries none of them, and the API server writes what it
// carries, so they would be gone from storage: every update counts, a label-only patch included,
// since a patch is applied to the template as read through v1alpha1.
//
// oldObject cannot tell: the API server reads the stored template through v1alpha1 for such a
// request, which prunes those fields, and only then converts it to v1alpha2 for this webhook. So the
// stored template is read here at v1alpha2, past the cache. It has to be the version oldObject
// shows, or this would judge a template the request does not replace; a changed one is refused as a
// conflict, the answer clients retry on.
func (v *validator) v1alpha1Erasure(ctx context.Context, req admission.Request, template *v1alpha2.ProjectTemplate) admission.Response {
	if req.Operation != admissionv1.Update || !throughVersion(req, v1alpha1.SchemeGroupVersion) {
		return admission.Allowed("")
	}

	old := new(v1alpha2.ProjectTemplate)
	if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode the old project template: %w", err))
	}

	stored := new(v1alpha2.ProjectTemplate)
	if err := v.reader.Get(ctx, client.ObjectKey{Name: template.Name}, stored); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("get the '%s' project template: %w", template.Name, err))
	}
	if stored.ResourceVersion != old.ResourceVersion {
		return admission.Response{AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:   http.StatusConflict,
				Reason: metav1.StatusReasonConflict,
				Message: fmt.Sprintf("the '%s' project template changed while this request through %s was checked; retry the request",
					template.Name, v1alpha1.SchemeGroupVersion),
			},
		}}
	}

	if fields := stored.Spec.V1alpha2OnlyFields(); len(fields) > 0 {
		return admission.Denied(fmt.Sprintf(
			"the '%s' project template has fields that %s cannot describe: %s; a request through %s would erase them, "+
				"so change the template through %s",
			template.Name, v1alpha1.SchemeGroupVersion, strings.Join(fields, ", "), v1alpha1.SchemeGroupVersion, v1alpha2.SchemeGroupVersion))
	}
	return admission.Allowed("")
}

// throughVersion reports whether the request was made through the version of ProjectTemplate. The
// admission rule lists v1alpha2 only, and with matchPolicy: Equivalent the API server delivers a
// request through v1alpha1 converted to v1alpha2: Kind names the version the webhook gets, and
// RequestKind the one the client used.
func throughVersion(req admission.Request, version schema.GroupVersion) bool {
	kind := req.RequestKind
	if kind == nil {
		kind = &req.Kind
	}
	return kind.Group == version.Group && kind.Version == version.Version
}

// newResourcesTemplate refuses a request through deckhouse.io/v1alpha2 that gives a template a
// resourcesTemplate it did not have: a create with a non-blank one, and an update that sets one on a
// template whose stored resourcesTemplate is blank. The field is deprecated and stays for the
// templates written through deckhouse.io/v1alpha1, which work as before: their text can be edited
// through either version, and a request through v1alpha1 may still set one. A Helm-text template with
// fields of v1alpha2 alone, such as one a backup restore or a GitOps bootstrap applies as v1alpha2,
// therefore takes two writes, which the refusal names. For a request through v1alpha2, oldObject is
// the stored template read at v1alpha2, which has the field.
func newResourcesTemplate(req admission.Request, template *v1alpha2.ProjectTemplate) admission.Response {
	if !template.Spec.HasResourcesTemplate() || !throughVersion(req, v1alpha2.SchemeGroupVersion) {
		return admission.Allowed("")
	}
	if req.Operation == admissionv1.Update {
		old := new(v1alpha2.ProjectTemplate)
		if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode the old project template: %w", err))
		}
		if old.Spec.HasResourcesTemplate() {
			return admission.Allowed("")
		}
	}
	return admission.Denied(fmt.Sprintf(
		"the '%s' project template cannot get a new resourcesTemplate through %s: the field is deprecated and stays only "+
			"for the templates written through %s, so describe the objects with the structured fields, and the ones "+
			"no field describes with manifests; a template that still needs a Helm text is created through %s, "+
			"and its title, resources or grantPolicies are added through %s afterwards",
		template.Name, v1alpha2.SchemeGroupVersion, v1alpha1.SchemeGroupVersion, v1alpha1.SchemeGroupVersion, v1alpha2.SchemeGroupVersion))
}

// resourcesTemplateRemoval refuses an update that empties the resourcesTemplate of a template that
// projects use and sets no field that renders objects in its place, such as a write-back by a client
// that does not know the field. Every project of the template would then switch to a render of
// little more than its namespace, and the project controller would leave every other object of the
// text in place with nobody managing it (manager/project/transition.go). An update that rewrites the
// template with structured fields or manifests in the same request is let through, and so is one of a
// template no project uses. Such a rewrite, like moving the projects to a structured template, leaves
// in place every object of the text that the new render lacks, except the rules and quotas the
// standard fields replace, until manifests takes it over (manager/project/manifests.go), which the
// refusal says.
func (v *validator) resourcesTemplateRemoval(ctx context.Context, req admission.Request, template *v1alpha2.ProjectTemplate) admission.Response {
	if req.Operation != admissionv1.Update || template.Spec.HasResourcesTemplate() || len(template.Spec.ObjectFields()) > 0 {
		return admission.Allowed("")
	}

	old := new(v1alpha2.ProjectTemplate)
	if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode the old project template: %w", err))
	}
	if !old.Spec.HasResourcesTemplate() {
		return admission.Allowed("")
	}

	user, err := v.projectUsing(ctx, template.Name)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if user == "" {
		return admission.Allowed("")
	}

	return admission.Denied(fmt.Sprintf(
		"the '%s' project template is used in the '%s' project, and without resourcesTemplate it sets no field that renders objects, "+
			"so its projects would no longer manage the objects the Helm template rendered; "+
			"set the structured fields or manifests in the same request or move the projects to another template first, "+
			"and a switch leaves every object of the Helm template that the new render lacks in place "+
			"and lists it in status.keptObjects of the project, except the AuthorizationRule and ResourceQuota objects "+
			"that the standard fields of the project replace, until manifests renders it under the same kind, namespace and name",
		template.Name, user))
}

// projectUsing returns the name of a project that uses the template, or "" when none does. A project
// uses it when its spec names the template or its template label does. The controller sets the label
// after the project is written, so a project created on the template or moved to it a moment ago
// carries only the spec, and one moved away from it a moment ago still renders from it with only the
// label. The list is read from the API server, so a project the cache has not seen yet counts too.
func (v *validator) projectUsing(ctx context.Context, template string) (string, error) {
	projects := new(v1alpha3.ProjectList)
	if err := v.reader.List(ctx, projects); err != nil {
		return "", fmt.Errorf("list the projects: %w", err)
	}
	for _, project := range projects.Items {
		if project.Spec.ProjectTemplateName == template || project.Labels[v1alpha3.ResourceLabelTemplate] == template {
			return project.Name, nil
		}
	}
	return "", nil
}

// previousNamespaceLabels returns the literal namespaceMetadata.labels of the template an update
// replaces: empty on create, or when the old template took them from a parameter.
func previousNamespaceLabels(req admission.Request) (map[string]string, error) {
	if req.Operation != admissionv1.Update {
		return map[string]string{}, nil
	}
	old := new(v1alpha2.ProjectTemplate)
	if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
		return nil, fmt.Errorf("decode the old project template: %w", err)
	}
	if old.Spec.NamespaceMetadata == nil {
		return map[string]string{}, nil
	}
	if labels, isLiteral := old.Spec.NamespaceMetadata.Labels.Literal(); isLiteral {
		return labels, nil
	}
	return map[string]string{}, nil
}

// validateInlineGrantSelectors applies the ClusterResourceGrantPolicy selector rules to the inline
// spec.resources entries of a template.
//
// The template schema accepts selector shapes the policy schema refuses -- an empty selector, or a
// matchExpressions entry the selector library cannot compile. The templategrants controller copies
// these entries verbatim into a managed ClusterResourceGrantPolicy, whose creation is then rejected
// by admission on every attempt, with nothing on the template to say why. Refusing the template is
// the only place the author finds out.
func validateInlineGrantSelectors(template *v1alpha2.ProjectTemplate) admission.Response {
	for i := range template.Spec.Resources {
		entry := &template.Spec.Resources[i]
		for _, sel := range []struct {
			field    string
			selector *metav1.LabelSelector
		}{
			{"allowedSelector", entry.AllowedSelector},
			{"deniedSelector", entry.DeniedSelector},
		} {
			if sel.selector == nil {
				continue
			}
			if len(sel.selector.MatchLabels) == 0 && len(sel.selector.MatchExpressions) == 0 {
				return admission.Denied(fmt.Sprintf(
					"the '%s' project template field 'spec.resources[%d].%s' (%s) is an empty selector: a selector must carry "+
						"matchLabels or matchExpressions; omit the field to grant nothing through it",
					template.Name, i, sel.field, entry.ResourceName))
			}
			if _, err := metav1.LabelSelectorAsSelector(sel.selector); err != nil {
				return admission.Denied(fmt.Sprintf(
					"the '%s' project template field 'spec.resources[%d].%s' (%s) is not a valid selector: %v",
					template.Name, i, sel.field, entry.ResourceName, err))
			}
		}
	}
	return admission.Allowed("")
}

// validateGrantPolicies enforces the library convention for spec.grantPolicies: every referenced
// ClusterResourceGrantPolicy must exist and must carry neither a projectSelector nor a
// namespaceSelector. A policy with a selector is already bound to its own set of namespaces (or is a
// controller-managed materialized policy), so referencing it from a template would double-bind it.
func (v *validator) validateGrantPolicies(ctx context.Context, template *v1alpha2.ProjectTemplate) admission.Response {
	for _, name := range template.Spec.GrantPolicies {
		policy := new(grantsv1alpha2.ClusterResourceGrantPolicy)
		if err := v.reader.Get(ctx, client.ObjectKey{Name: name}, policy); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf(
					"the '%s' project template references ClusterResourceGrantPolicy '%s' which does not exist",
					template.Name, name))
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !policy.Spec.IsLibrary() {
			return admission.Denied(fmt.Sprintf(
				"the '%s' project template references ClusterResourceGrantPolicy '%s' which has a projectSelector or a namespaceSelector; "+
					"grantPolicies may only reference library policies (without either selector)",
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

// literalValues are the values the fixed-set fields accept as literals. They live here and nowhere
// else: both fields are untyped in the CRD schema (a string or a fromParam object, under
// x-kubernetes-preserve-unknown-fields), and the API server builds no CEL declarations for an
// untyped field, so the schema cannot carry the sets as a validation rule.
var literalValues = map[string][]string{
	"podSecurityStandard": {v1alpha2.PodSecurityStandardPrivileged, v1alpha2.PodSecurityStandardBaseline, v1alpha2.PodSecurityStandardRestricted},
	"networkPolicy.mode":  {v1alpha2.NetworkPolicyModeIsolated, v1alpha2.NetworkPolicyModeNotRestricted},
}

// validateLiterals checks every fixed-set field that holds a literal (a {fromParam} reference is
// checked against the parameters schema instead, see FromParamRefs).
func validateLiterals(template *v1alpha2.ProjectTemplate) error {
	check := func(field string, param v1alpha2.Param[string]) error {
		value, isLiteral := param.Literal()
		if !isLiteral {
			return nil
		}
		allowed := literalValues[field]
		if slices.Contains(allowed, value) {
			return nil
		}
		return fmt.Errorf("field '%s' must be one of %s or a {fromParam: <name>} reference, got '%s'", field, strings.Join(allowed, ", "), value)
	}
	if err := check("podSecurityStandard", template.Spec.PodSecurityStandard); err != nil {
		return err
	}
	if template.Spec.NetworkPolicy != nil {
		if err := check("networkPolicy.mode", template.Spec.NetworkPolicy.Mode); err != nil {
			return err
		}
	}
	return nil
}
