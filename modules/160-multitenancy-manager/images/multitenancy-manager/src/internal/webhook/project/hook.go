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

package project

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	projectmanager "controller/internal/manager/project"
	"controller/internal/naming"
	"controller/internal/validate"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// ownLabelChange checks an update against the labels the controller stamps on every Project. Grant
// policies select projects by them, so pointing projects.deckhouse.io/project-template at a template the
// project does not use is refused, as it would move the project into or out of policies. Removing either
// label is let through with a warning that names it. A manifest that replaces the whole Project (kubectl
// replace, Argo CD with Replace=true) does not carry them, and the controller puts them back, as
// ownLabelsDrifted wakes it. Keeping the old value, or setting the one the controller is about to stamp,
// is fine.
func ownLabelChange(old, project *v1alpha3.Project) (string, []string) {
	if value, ok := project.Labels[v1alpha3.ResourceLabelTemplate]; ok &&
		value != project.Spec.ProjectTemplateName && value != old.Labels[v1alpha3.ResourceLabelTemplate] {
		return fmt.Sprintf("the %s label of the %q project must be its template %q, got %q",
			v1alpha3.ResourceLabelTemplate, project.Name, project.Spec.ProjectTemplateName, value), nil
	}
	var warnings []string
	for _, key := range []string{v1alpha3.ResourceLabelProject, v1alpha3.ResourceLabelTemplate} {
		_, had := old.Labels[key]
		_, has := project.Labels[key]
		if had && !has {
			warnings = append(warnings, fmt.Sprintf(
				"the %s label of the %q project is set by the multitenancy-manager controller, which puts it back", key, project.Name))
		}
	}
	return "", warnings
}

func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projects", hook)
}

type validator struct {
	client client.Client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	project := new(v1alpha3.Project)
	if err := yaml.Unmarshal(req.Object.Raw, project); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Only the controller/Deckhouse may perform privileged operations: auto-wrapping an existing
	// namespace into a managed-by-namespace project (Create) and editing a managed-by-namespace
	// project (Update). Cluster administrators are not privileged here: system:masters goes through
	// this handler like any user (the webhook's matchConditions skip only platform components and
	// system:sudouser, see templates/admission/validation.yaml).
	privileged := req.UserInfo.Username == rolebindingwebhook.ControllerServiceAccount ||
		req.UserInfo.Username == rolebindingwebhook.DeckhouseServiceAccount

	// The platform's virtual projects -- default and deckhouse, on the "virtual" template -- have no
	// namespace of their own and are created by the controller alone. Both their names and their
	// template are reserved for it: a user project on that template is handled as virtual by the
	// project controller (no namespace, Deployed) but looks like an ordinary project to everything
	// keyed on the virtual-project label, so the PRB/CPRB fan-out kept failing on a namespace that
	// never exists and no ClusterProjectRoleBinding in the cluster could reach Ready; and a user
	// project under one of the two names would take the place the controller expects to fill.
	virtualName := project.Name == projectmanager.DefaultProjectName || project.Name == projectmanager.DeckhouseProjectName
	if !privileged && (virtualName || project.Spec.ProjectTemplateName == projectmanager.VirtualTemplate) {
		return admission.Denied(fmt.Sprintf("the %q and %q project names and the %q project template are reserved for the platform's virtual projects",
			projectmanager.DefaultProjectName, projectmanager.DeckhouseProjectName, projectmanager.VirtualTemplate))
	}

	// projects.deckhouse.io/project names the project it is on: grant policies select projects by it
	// (projectSelector), so a Project must not claim another project's name. The controller stamps it;
	// a manifest may leave it out or repeat the project's own name.
	if value, ok := project.Labels[v1alpha3.ResourceLabelProject]; ok && !privileged && value != project.Name {
		return admission.Denied(fmt.Sprintf("the %s label of the %q project must be the project name, got %q",
			v1alpha3.ResourceLabelProject, project.Name, value))
	}

	// The warnings of the checks below go out with whichever response allows the request.
	var warnings []string

	var old *v1alpha3.Project
	if req.Operation == admissionv1.Update && !privileged {
		old = new(v1alpha3.Project)
		if err := yaml.Unmarshal(req.OldObject.Raw, old); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		reason, labelWarnings := ownLabelChange(old, project)
		if reason != "" {
			return admission.Denied(reason)
		}
		warnings = append(warnings, labelWarnings...)
	}

	if req.Operation == admissionv1.Create {
		// pass the platform's virtual projects: the controller creates them without a namespace
		if virtualName {
			return admission.Allowed("").WithWarnings(warnings...)
		}

		if strings.HasPrefix(project.Name, "d8-") || strings.HasPrefix(project.Name, "kube-") {
			return admission.Denied("Projects cannot start with 'd8-' or 'kube-'")
		}

		// The project's main namespace is named after the project, so a single Get is enough; a full
		// namespace List would scan the whole cluster on every project create.
		namespace := new(corev1.Namespace)
		switch err := v.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace); {
		case err == nil:
			// The controller adopts every namespace that belongs to no project, so a same-name
			// namespace is expected when the request comes from the controller itself. A user still
			// cannot claim an existing namespace by creating a project over it.
			if !privileged {
				return admission.Denied(fmt.Sprintf("The '%s' project cannot be created, a namespace with its name exists", project.Name))
			}
		case !apierrors.IsNotFound(err):
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// prefix collisions: the "<project>-*" name space is reserved for the additional namespaces
		// of an existing project, so neither "foo-bar" (when "foo" exists) nor "foo" (when "foo-bar"
		// exists) may be created.
		projects := new(v1alpha3.ProjectList)
		if err := v.client.List(ctx, projects); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, existing := range projects.Items {
			if existing.Name == project.Name {
				continue
			}
			if strings.HasPrefix(project.Name, existing.Name+"-") {
				return admission.Denied(fmt.Sprintf(
					"project name %q conflicts with project %q: %q-* names are reserved for additional namespaces of project %q",
					project.Name, existing.Name, existing.Name, existing.Name))
			}
			if strings.HasPrefix(existing.Name, project.Name+"-") {
				return admission.Denied(fmt.Sprintf(
					"project name %q conflicts with project %q: %q-* names are reserved for additional namespaces of project %q",
					project.Name, existing.Name, project.Name, project.Name))
			}
		}
	}

	// validate the standard fields (cheap checks before the OpenAPI validation)
	if denied := validateStandardFields(project); denied != "" {
		return admission.Denied(denied)
	}

	if req.Operation == admissionv1.Update {
		// pass triggered projects
		if privileged {
			if annotations := project.Annotations; annotations != nil {
				if require, ok := annotations[v1alpha3.ProjectAnnotationRequireSync]; ok && require == "true" {
					return admission.Allowed("").WithWarnings(warnings...)
				}
			}
		}

		// pass error projects (the status subresource is controller-managed). Gated to privileged
		// requests so the controller can keep re-reconciling an already-erroring project, while a
		// non-privileged user editing such a project still goes through full template/render
		// validation instead of slipping further invalid spec edits past admission.
		if privileged && project.Status.State == v1alpha3.ProjectStateError {
			return admission.Allowed("").WithWarnings(warnings...).WithWarnings("The project skip validation due to the status")
		}
	}

	// skip project with empty template
	if project.Spec.ProjectTemplateName == "" {
		return admission.Allowed("").WithWarnings(warnings...)
	}

	template, err := v.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if template == nil {
		return admission.Allowed("").WithWarnings(warnings...).WithWarnings("The project template not found")
	}

	// validate the project parameters against the template schema. The render itself is not
	// rehearsed here: a structured template renders from its fields and the resolved parameters,
	// and both are validated on their own -- the fields by the template webhook, the parameters
	// just above.
	if err = validate.Project(project, template); err != nil {
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %v", project.Name, err))
	}

	// The namespace labels the template takes from the parameters (the built-in templates wire
	// namespaceMetadata.labels to namespace.labels) cannot carry a label the module owns: the renderer
	// drops it, and the project would lack what its user asked for without a word. The template
	// webhook checks a literal; a parameter is known only here.
	//
	// A project written before this check may already carry such a label. Refusing every later edit
	// of it for that label would lock the project, so an update on the same template is refused only
	// for a label it sets anew, and the ones it keeps are named in a warning. The controller and
	// Deckhouse never set one, and their updates are not checked.
	if privileged {
		return admission.Allowed("").WithWarnings(warnings...)
	}
	labels, ref := namespaceLabels(project, template)
	var previous map[string]string
	if old != nil && old.Spec.ProjectTemplateName == project.Spec.ProjectTemplateName {
		previous, _ = namespaceLabels(old, template)
	}
	set, kept := naming.SplitModuleOwnedLabels(labels, previous)
	if len(set) > 0 {
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: the parameter '%s' sets namespace labels the module owns: %s",
			project.Name, ref, strings.Join(naming.ModuleOwnedLabelsIn(set), "; ")))
	}
	if len(kept) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"the parameter '%s' of the '%s' project sets namespace labels the module owns, and the project does not apply them: %s; remove them from the parameter",
			ref, project.Name, strings.Join(naming.ModuleOwnedLabelsIn(kept), "; ")))
	}

	return admission.Allowed("").WithWarnings(warnings...)
}

// namespaceLabels resolves namespaceMetadata.labels of the template against the project parameters,
// as the renderer does, and names the parameter they come from. Nil when the template takes no
// labels from the parameters.
func namespaceLabels(project *v1alpha3.Project, template *v1alpha2.ProjectTemplate) (map[string]string, string) {
	if template.Spec.NamespaceMetadata == nil || template.Spec.NamespaceMetadata.Labels.Ref() == "" {
		return nil, ""
	}
	schema, err := validate.LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return nil, "" // validate.Project refuses a template whose schema does not load
	}
	labels, ok, err := template.Spec.NamespaceMetadata.Labels.Resolve(validate.MergeDefaults(schema, project.Spec.Parameters))
	if err != nil || !ok {
		return nil, ""
	}
	return labels, template.Spec.NamespaceMetadata.Labels.Ref()
}

// byteQuantityUnitRE matches a Kubernetes Quantity that carries an explicit byte-scale unit.
// Milli/micro/nano suffixes (m/u/n) and bare numbers are intentionally excluded: for memory and
// storage a bare "5" is 5 bytes, which is almost always a mistake.
var byteQuantityUnitRE = regexp.MustCompile(`(Ki|Mi|Gi|Ti|Pi|Ei|[kMGTPE])$`)

// validateStandardFields performs cheap validation of the Project standard fields. It returns a
// non-empty denial message when the project is invalid.
func validateStandardFields(project *v1alpha3.Project) string {
	for _, admin := range project.Spec.Administrators {
		if admin.Kind != "User" && admin.Kind != "Group" {
			return fmt.Sprintf("administrator %q has invalid kind %q: must be User or Group", admin.Name, admin.Kind)
		}
		if admin.Name == "" {
			return "administrator name must not be empty"
		}
	}
	if msg := validateQuotaByteUnits(project.Spec.Quota); msg != "" {
		return msg
	}
	return ""
}

// resourceNameRequiresByteUnit reports whether a ResourceQuota hard key is a memory/storage
// quantity that must include an explicit unit suffix (Gi, Mi, …). hugepages-* are page counts and
// are left alone; cpu/pods/count/* stay numeric.
func resourceNameRequiresByteUnit(name corev1.ResourceName) bool {
	s := string(name)
	if strings.Contains(s, "hugepages-") {
		return false
	}
	return strings.HasSuffix(s, "memory") || strings.HasSuffix(s, "storage")
}

func hasByteUnitSuffix(q resource.Quantity) bool {
	return byteQuantityUnitRE.MatchString(q.String())
}

func validateQuotaByteUnits(quota corev1.ResourceList) string {
	for name, quantity := range quota {
		if !resourceNameRequiresByteUnit(name) {
			continue
		}
		if hasByteUnitSuffix(quantity) {
			continue
		}
		return fmt.Sprintf(
			"%s must include a unit suffix, e.g. 2Gi (bare numbers are interpreted as bytes)",
			name,
		)
	}
	return ""
}

// projectTemplateByName reads the template at v1alpha2, the served version. Asking for v1alpha1 --
// which this did -- worked only while that version was served: the apiserver converted the stored
// object on every call, and once v1alpha1 stopped being served the lookup began failing with "no
// matches for kind ProjectTemplate in version deckhouse.io/v1alpha1", which denied every project write.
func (v *validator) projectTemplateByName(ctx context.Context, name string) (*v1alpha2.ProjectTemplate, error) {
	template := new(v1alpha2.ProjectTemplate)
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get the '%s' project template: %w", name, err)
	}

	return template, nil
}
