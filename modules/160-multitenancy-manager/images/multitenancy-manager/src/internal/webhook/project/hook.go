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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	projectmanager "controller/internal/manager/project"
	"controller/internal/naming"
	"controller/internal/rolebinding"
	"controller/internal/validate"
	projectnamespacewebhook "controller/internal/webhook/projectnamespace"
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

func Register(runtimeManager manager.Manager, helmClient *helm.Client) {
	hook := &webhook.Admission{Handler: &validator{
		client:     runtimeManager.GetClient(),
		helmClient: helmClient,
		reader:     runtimeManager.GetAPIReader(),
	}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projects", hook)
}

type validator struct {
	client     client.Client
	helmClient *helm.Client
	// reader is the direct API reader for the role bindings that still name the ServiceAccounts of
	// the project's namespace, whose informers the cache would start inside the request on a replica
	// that is not the leader, and for the ProjectNamespaces that claim the project's name, which the
	// cache may not have seen yet.
	reader client.Reader
}

// apiReader is the reader for the role bindings and the ProjectNamespace claims: the direct API reader
// when the validator has one, which the webhook server always gives it, and the client otherwise.
func (v *validator) apiReader() client.Reader {
	if v.reader != nil {
		return v.reader
	}
	return v.client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// The request carries JSON, and it is read as JSON. A YAML decoder reads a JSON document too,
	// but the one sigs.k8s.io/yaml uses folds an unescaped U+0085 in a string into a space, and the
	// API server escapes U+2028 and U+2029 only: an administrator name would pass the line-break
	// check below and be stored with its U+0085.
	project := new(v1alpha3.Project)
	if err := json.Unmarshal(req.Object.Raw, project); err != nil {
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
		if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
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

		// A ProjectNamespace claims "<its project>-<spec.name>" as soon as it is created, and its
		// namespace appears only when the controller gets to it. A project under that name would take
		// the namespace from the ProjectNamespace, so the claim counts as the namespace itself. The
		// claims are read from the API server: a ProjectNamespace created just before the project, as
		// a GitOps sync applies both, may not be in the cache yet.
		claim, err := projectnamespacewebhook.Claiming(ctx, v.apiReader(), project.Name)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if claim != nil {
			return admission.Denied(fmt.Sprintf("The '%s' project cannot be created, the '%s' ProjectNamespace of the '%s' project claims a namespace with its name",
				project.Name, claim.Name, claim.Namespace))
		}

		// A binding of another project, or a cluster-wide one, can still name the ServiceAccounts of a
		// namespace under this name that is gone, and the project would hand them to its creator. The
		// controller adopts only namespaces that exist already, so its creates are not checked.
		if !privileged {
			reason, err := rolebinding.ServiceAccountGrantConflict(ctx, v.apiReader(), project.Name, project.Name)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if reason != "" {
				return admission.Denied(fmt.Sprintf("The '%s' project cannot be created, %s", project.Name, reason))
			}
		}

		// Only the names ProjectNamespaces claim are reserved. Any other "<project>-*" name is a
		// project of its own, which reads like an additional namespace of that project and is not
		// one; the warning says so while the author can still choose a ProjectNamespace instead.
		projects := new(v1alpha3.ProjectList)
		if err := v.client.List(ctx, projects); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		warnings = append(warnings, nameWarnings(project.Name, projects.Items)...)
	}

	// validate the standard fields (cheap checks before the OpenAPI validation); an update is
	// checked against the project it replaces
	spellings, err := quotaSpellings(req.Object.Raw)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	v1alpha2Create := req.Operation == admissionv1.Create && writtenThroughV1alpha2(req)
	denied, standardFieldWarnings := validateStandardFields(project, spellings, storedProject(req), v1alpha2Create)
	if denied != "" {
		return admission.Denied(denied)
	}
	warnings = append(warnings, standardFieldWarnings...)

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

	// validate the project parameters against the template schema
	if err = validate.Project(project, template); err != nil {
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %v", project.Name, err))
	}

	// validate.Project leaves out an administrators or resourceQuota parameter the template schema
	// does not declare, so that a project the conversion left one in keeps reconciling. Which of them
	// a write may bring in is decided here. The controller and Deckhouse never set one, and their
	// writes are not checked.
	if !privileged {
		denied, leftoverWarnings := undeclaredLeftovers(project, old, template, writtenThroughV1alpha2(req))
		if denied != "" {
			return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %s", project.Name, denied))
		}
		warnings = append(warnings, leftoverWarnings...)
	}

	// Rehearse the Helm render of a resourcesTemplate. It is free-form text, and what it makes of the
	// parameters shows only in the render: a text that does not render, or a parameter that becomes
	// manifest structure instead of a value, is refused here and not first when the release is
	// applied. An object put into a namespace outside the project is moved to the main namespace,
	// which is worth a warning, not a refusal. The structured fields are not rehearsed: they render
	// from the resolved parameters, and both are validated on their own, the fields by the template
	// webhook and the parameters just above.
	if template.Spec.HasResourcesTemplate() {
		if err = v.helmClient.ValidateRender(project, projectmanager.LegacyTemplate(template)); err != nil {
			if !errors.Is(err, helm.ErrNamespaceOverride) {
				return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %v", project.Name, err))
			}
			warnings = append(warnings, err.Error())
		}
	}

	// manifests is free-form text too, rendered with the same values and the same check of the
	// parameters, so it is rehearsed the same way, and a text that does not render or a parameter that
	// becomes structure is refused here rather than first in the ManifestsApplied condition. Whether
	// the objects may be applied is the controller's to say: it depends on the namespaces of the
	// project and on the kinds the cluster serves.
	if template.Spec.HasManifests() {
		_, err = v.helmClient.RenderManifests(project, template.Spec.Manifests, template.Spec.ParametersSchema.OpenAPIV3Schema)
		if err != nil {
			return admission.Denied(fmt.Sprintf("The project '%s' is invalid: the manifests of the '%s' project template: %v", project.Name, template.Name, err))
		}
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

// writtenThroughV1alpha2 reports whether the request writes the project as deckhouse.io/v1alpha2. The
// webhook gets every request at v1alpha3 (matchPolicy: Equivalent), and RequestKind names the version
// the client wrote.
func writtenThroughV1alpha2(req admission.Request) bool {
	return req.RequestKind != nil &&
		req.RequestKind.Group == v1alpha2.SchemeGroupVersion.Group && req.RequestKind.Version == v1alpha2.SchemeGroupVersion.Version
}

// leftover describes the administrators or the resourceQuota parameter for the messages of
// undeclaredLeftovers.
type leftover struct {
	// effect says what the parameter does not do on a template that does not declare it
	effect string
	// field is the standard field that does it, and holds what
	field, holds string
	// lifted is the shape the v1alpha2 -> v1alpha3 conversion lifts into the field, in the words of
	// liftable_administrators and liftable_quota of webhooks/conversion/projects
	lifted string
}

var leftovers = map[string]leftover{
	"administrators": {
		effect: "grants no access",
		field:  "spec.administrators",
		holds:  "administrators",
		lifted: "a non-empty list whose items hold only a subject, User or Group, and a name that is not empty and has no ASCII control character",
	},
	"resourceQuota": {
		effect: "sets no quota",
		field:  "spec.quota",
		holds:  "a quota",
		lifted: `a non-empty object whose top-level keys do not start with "requests." or "limits.", ` +
			`whose requests and limits, if they are objects, are not empty, ` +
			`and whose other values and the values under requests and limits are quantities such as 2, 500m or 1Gi other than the number 0, ` +
			`which set no limit; leave such a limit out to keep it unset, since the string "0" is a hard limit of zero`,
	},
}

// undeclaredLeftovers checks the administrators and resourceQuota parameters that the parametersSchema
// of the template does not declare (see validate.UndeclaredLeftovers). There they grant no access and
// set no quota. It returns a non-empty denial message for a value the write may not bring in, and the
// warnings for the others. old is the stored project of an update and nil otherwise.
//
// A value that grants or limits nothing passes with a warning (see grantsOrLimitsNothing). So does a
// value an update keeps as the stored project holds it, on a move to another template as well: the
// conversion leaves such values in projects of the previous release, whose built-in templates
// declared the two, and refusing them would refuse every later write of the project, a re-apply of
// the unchanged manifest included. The value is kept whatever template the stored project is on, so
// a move from a template that declares the parameter, where the value could grant access or set a
// quota, keeps it with a warning as well. Any other value is refused. The writer meant it to grant or
// limit something, and it would not, while a value a v1alpha3 client puts there in a shape the
// conversion lifts would take effect on the next write through v1alpha2. A write through v1alpha2 has
// only the parameters, so its messages say which shape the conversion lifts into the standard field.
func undeclaredLeftovers(
	project, old *v1alpha3.Project,
	template *v1alpha2.ProjectTemplate,
	throughV1alpha2 bool,
) (string, []string) {
	schema, err := validate.LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return "", nil // validate.Project refuses a template whose schema does not load
	}

	var warnings []string
	for _, name := range validate.UndeclaredLeftovers(project.Spec.Parameters, schema) {
		described := leftovers[name]
		noEffect := fmt.Sprintf("has no effect on the '%s' project template, whose parametersSchema does not declare it", template.Name)
		var lifted string
		if throughV1alpha2 {
			lifted = fmt.Sprintf(", which a deckhouse.io/v1alpha2 manifest sets through this parameter when it is %s", described.lifted)
		}

		switch {
		case grantsOrLimitsNothing(name, project.Spec.Parameters[name]):
			warnings = append(warnings, fmt.Sprintf("the parameter '%s' of the '%s' project %s; remove it from the parameters", name, project.Name, noEffect))
		case old != nil && sameParameter(old, project, name):
			warnings = append(warnings, fmt.Sprintf(
				"the parameter '%s' of the '%s' project %s, and %s; it is accepted because it did not change, and %s is the place for %s%s",
				name, project.Name, noEffect, described.effect, described.field, described.holds, lifted))
		default:
			return fmt.Sprintf("the parameter '%s' %s, and %s; set %s instead%s",
				name, noEffect, described.effect, described.field, lifted), nil
		}
	}
	return "", warnings
}

// grantsOrLimitsNothing reports whether a value of the administrators or resourceQuota parameter
// grants or limits nothing: administrators that are null or an empty list, or a quota that is null or
// holds no value but nulls and the number 0, at the top or under requests and limits. The templates
// of the previous release skipped the number 0 with "with", and the conversion lifts no quota that
// holds it, so it never becomes a limit. The string "0" is not among these: in a quota the conversion
// lifts, it becomes a hard limit of zero in spec.quota. The request is read with encoding/json, which
// gives a JSON number as a float64.
func grantsOrLimitsNothing(name string, value any) bool {
	if value == nil {
		return true
	}
	if name == "administrators" {
		list, isList := value.([]any)
		return isList && len(list) == 0
	}

	quota, isObject := value.(map[string]any)
	if !isObject {
		return false
	}
	limitsNothing := func(quantity any) bool {
		number, isNumber := quantity.(float64)
		return quantity == nil || isNumber && number == 0
	}
	for key, entry := range quota {
		nested, isObject := entry.(map[string]any)
		if isNesting := isObject && (key == "requests" || key == "limits"); !isNesting {
			if !limitsNothing(entry) {
				return false
			}
			continue
		}
		for _, quantity := range nested {
			if !limitsNothing(quantity) {
				return false
			}
		}
	}
	return true
}

// byteQuantityUnitRE matches a Kubernetes Quantity written with an explicit byte-scale unit.
// Milli/micro/nano suffixes (m/u/n), exponents (1e3, 1E6) and bare numbers are intentionally
// excluded: for memory and storage a bare "5" is 5 bytes, which is almost always a mistake.
var byteQuantityUnitRE = regexp.MustCompile(`(Ki|Mi|Gi|Ti|Pi|Ei|[kMGTPE])$`)

// storedProject returns the project an update replaces, and nil for any other operation. A stored
// project that does not decode counts as none, so the update is checked in full: it may be the one
// that repairs the project.
func storedProject(req admission.Request) *v1alpha3.Project {
	if req.Operation != admissionv1.Update || len(req.OldObject.Raw) == 0 {
		return nil
	}
	old := new(v1alpha3.Project)
	if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
		return nil
	}
	return old
}

// quotaSpellings returns the spec.quota values of a raw Project as the request writes them, read the
// way Quantity reads them: the text of a JSON string without the spaces around it, and the literal
// of a JSON number, which never ends in a unit. The parsed Quantity keeps only its canonical form,
// and in it a bare 1000 reads 1k.
func quotaSpellings(raw []byte) (map[corev1.ResourceName]string, error) {
	var object struct {
		Spec struct {
			Quota map[corev1.ResourceName]json.RawMessage `json:"quota"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("read spec.quota: %w", err)
	}

	spellings := make(map[corev1.ResourceName]string, len(object.Spec.Quota))
	for name, value := range object.Spec.Quota {
		var text string
		if json.Unmarshal(value, &text) != nil {
			text = string(value) // a JSON number
		}
		spellings[name] = strings.TrimSpace(text)
	}
	return spellings, nil
}

// validateStandardFields performs cheap validation of the Project standard fields. It returns a
// non-empty denial message when the project is invalid, and the warnings for a valid one.
// spellings are the quota values as the request writes them (see quotaSpellings). old is the stored
// project of an update and nil otherwise. An update is checked only for what it changes: the
// up-conversion lifts into the standard fields what an older release accepted, a memory quota without
// a unit for one, and such a project has to stay editable. v1alpha2Create is true for a create through
// deckhouse.io/v1alpha2, the version the manifests of that release are written in, and it lets a quota
// value without a unit through as well (see validateQuotaByteUnits).
func validateStandardFields(
	project *v1alpha3.Project,
	spellings map[corev1.ResourceName]string,
	old *v1alpha3.Project,
	v1alpha2Create bool,
) (string, []string) {
	// The v1alpha2 version of a project holds a standard field in the parameter it is lifted from, so
	// it has room for only one of the two: the down-conversion writes the field over the parameter,
	// and a v1alpha2 client that writes the project back loses the parameter. The up-conversion never
	// fills both, so they meet only when a v1alpha3 client sets them together. An empty list or map
	// counts as set, since the down-conversion writes it over the parameter as well. An update that
	// leaves both as the stored project has them is let through, so that a project stored with both,
	// by a writer this webhook skips or before it refused them, stays editable until one of them goes.
	if administratorsNextToParameter(project) {
		kept := administratorsNextToParameter(old) &&
			equality.Semantic.DeepEqual(old.Spec.Administrators, project.Spec.Administrators) &&
			sameParameter(old, project, "administrators")
		if !kept {
			return standardFieldAndParameter("administrators", "administrators"), nil
		}
	}
	if quotaNextToParameter(project) {
		kept := quotaNextToParameter(old) &&
			equality.Semantic.DeepEqual(old.Spec.Quota, project.Spec.Quota) &&
			sameParameter(old, project, "resourceQuota")
		if !kept {
			return standardFieldAndParameter("quota", "resourceQuota"), nil
		}
	}

	var stored v1alpha3.ProjectSpec
	if old != nil {
		stored = old.Spec
	}
	var warnings []string
	for _, admin := range project.Spec.Administrators {
		// The CRD pattern refuses C0 controls and DEL only. The C1 controls have no meaning in a
		// subject name either, and YAML ends a scalar on U+0085, U+2028 and U+2029: a Helm template
		// whose schema declares the administrators parameter gets the name there (see
		// validate.V1alpha2Parameters) and may substitute it unquoted. An administrator the stored
		// project already has passes with a warning, as a kept quota value does: the conversion lifts
		// the administrators parameter of a v1alpha2 project, whose schema may have let such a name
		// in, and refusing it would refuse every later write of the project. The controller still
		// refuses a render the name breaks.
		unsafeName := strings.ContainsFunc(admin.Name, unicode.IsControl) || helm.ContainsLineBreak(admin.Name)
		reserved := strings.HasPrefix(admin.Name, naming.ReservedUserPrefix)
		if slices.Contains(stored.Administrators, admin) {
			if unsafeName {
				warnings = append(warnings, fmt.Sprintf(
					"administrator name %q contains a control character or a line break; it is accepted because it did not change, but a new name must not contain one",
					admin.Name))
			}
			if reserved {
				warnings = append(warnings, reservedAdministratorWarning(admin.Name))
			}
			continue
		}
		if admin.Kind != "User" && admin.Kind != "Group" {
			return fmt.Sprintf("administrator %q has invalid kind %q: must be User or Group", admin.Name, admin.Kind), nil
		}
		if admin.Name == "" {
			return "administrator name must not be empty", nil
		}
		if unsafeName {
			return fmt.Sprintf("administrator name %q must not contain control characters or line breaks", admin.Name), nil
		}
		if reserved {
			return reservedAdministratorDenial(admin.Name), nil
		}
	}

	// The administrators parameter of the v1alpha2 layout reaches the subjects of the objects a Helm
	// text renders, which the controller applies with its own rights, so a user under the reserved
	// prefix is refused there too, and one the stored project already names passes with a warning.
	var storedParameter []v1alpha3.Administrator
	if old != nil {
		storedParameter = helm.ParameterAdministrators(old.Spec.Parameters)
	}
	for _, admin := range helm.ParameterAdministrators(project.Spec.Parameters) {
		if !strings.HasPrefix(admin.Name, naming.ReservedUserPrefix) {
			continue
		}
		if slices.Contains(storedParameter, admin) {
			warnings = append(warnings, reservedAdministratorWarning(admin.Name))
			continue
		}
		return reservedAdministratorDenial(admin.Name), nil
	}

	denied, quotaWarnings := validateQuotaByteUnits(project.Spec.Quota, spellings, stored.Quota, v1alpha2Create)
	if denied != "" {
		return denied, nil
	}
	return "", append(warnings, quotaWarnings...)
}

// reservedAdministratorDenial refuses an administrator under naming.ReservedUserPrefix. The controller
// acts as the users under it, and only the controller binds them: an administrator under it would get
// the user of another project the rights of an administrator of this one.
func reservedAdministratorDenial(name string) string {
	return fmt.Sprintf("administrator name %q must not start with %s, which starts the names of the users the multitenancy-manager controller acts as",
		name, naming.ReservedUserPrefix)
}

// reservedAdministratorWarning is the warning for an administrator under naming.ReservedUserPrefix that
// the stored project already names.
func reservedAdministratorWarning(name string) string {
	return fmt.Sprintf(
		"administrator name %q starts with %s, which starts the names of the users the multitenancy-manager controller acts as; "+
			"it is accepted because it did not change, but a new name must not start with it, so remove it",
		name, naming.ReservedUserPrefix)
}

// administratorsNextToParameter reports whether the project sets spec.administrators together with
// the parameter it is lifted from.
func administratorsNextToParameter(project *v1alpha3.Project) bool {
	return project != nil && project.Spec.Administrators != nil && hasParameter(project, "administrators")
}

// quotaNextToParameter reports whether the project sets spec.quota together with the parameter it is
// lifted from.
func quotaNextToParameter(project *v1alpha3.Project) bool {
	return project != nil && project.Spec.Quota != nil && hasParameter(project, "resourceQuota")
}

// sameParameter reports whether the two projects hold the same value under the parameter name.
func sameParameter(old, project *v1alpha3.Project, name string) bool {
	return equality.Semantic.DeepEqual(old.Spec.Parameters[name], project.Spec.Parameters[name])
}

// hasParameter reports whether the project parameters hold a value under the name. A null says
// nothing, and the down-conversion loses nothing by writing over it.
func hasParameter(project *v1alpha3.Project, name string) bool {
	value, found := project.Spec.Parameters[name]
	return found && value != nil
}

func standardFieldAndParameter(field, parameter string) string {
	return fmt.Sprintf("spec.%s and spec.parameters.%s cannot both be set, because the v1alpha2 version of the project has room for only one of them",
		field, parameter)
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

// hasByteUnitSuffix reports whether a quantity is written with a byte-scale unit. It takes the
// spelling, not the parsed Quantity: the canonical form of a bare 1000 is 1k and of a bare 5000000
// is 5M, both of which end in a unit.
func hasByteUnitSuffix(spelling string) bool {
	return byteQuantityUnitRE.MatchString(spelling)
}

// validateQuotaByteUnits refuses a memory or storage quota value written without a byte-scale unit.
// 0 reads the same in every unit and passes. A value equal to the one the stored quota of an update
// holds under the same key is let through with a warning instead: the v1alpha2 -> v1alpha3
// conversion carries the bare numbers of the resourceQuota parameter over into spec.quota, and
// refusing them would refuse every later write of such a project, a re-apply of the unchanged
// manifest included. A create through deckhouse.io/v1alpha2 (v1alpha2Create) gets the same warning
// for any such value. The resourceQuota parameter took bare numbers, and a restore from a backup or a
// GitOps tool that creates the project again writes the manifest as it was.
func validateQuotaByteUnits(
	quota corev1.ResourceList,
	spellings map[corev1.ResourceName]string,
	stored corev1.ResourceList,
	v1alpha2Create bool,
) (string, []string) {
	var warnings []string
	for _, name := range slices.Sorted(maps.Keys(quota)) {
		quantity := quota[name]
		if !resourceNameRequiresByteUnit(name) || quantity.IsZero() {
			continue
		}
		if hasByteUnitSuffix(spellings[name]) {
			continue
		}

		if previous, found := stored[name]; found && previous.Cmp(quantity) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s is %s without a unit suffix, which is interpreted as bytes; it is accepted because it did not change, but a new value must include a unit suffix, e.g. 2Gi",
				name,
				spellings[name],
			))
			continue
		}
		if v1alpha2Create {
			warnings = append(warnings, fmt.Sprintf(
				"%s is %s without a unit suffix, which is interpreted as bytes; it is accepted because the project is created through %s, "+
					"whose resourceQuota parameter took such a value, but a new value must include a unit suffix, e.g. 2Gi",
				name,
				spellings[name],
				v1alpha2.SchemeGroupVersion,
			))
			continue
		}

		return fmt.Sprintf(
			"%s must include a unit suffix, e.g. 2Gi (bare numbers are interpreted as bytes)",
			name,
		), nil
	}
	return "", warnings
}

// projectTemplateByName reads the template at v1alpha2, the storage version and the one that has every
// field. Asking for v1alpha1, which this once did, prunes the structured fields, and while v1alpha1
// was not served the lookup failed with "no matches for kind ProjectTemplate in version
// deckhouse.io/v1alpha1", which denied every project write.
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
