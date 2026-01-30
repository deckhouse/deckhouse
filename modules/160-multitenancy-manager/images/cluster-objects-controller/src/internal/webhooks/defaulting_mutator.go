package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ http.Handler = &DefaultingMutator{}

type DefaultingMutator struct {
	log             logr.Logger
	cl              client.Client
	dynamicCl       dynamic.Interface
	jsonpathFactory jsonpath.Factory
}

func NewDefaultingMutator(
	log logr.Logger,
	client client.Client,
	dynamicClient dynamic.Interface,
	jsonpathFactory jsonpath.Factory,
) *DefaultingMutator {
	return &DefaultingMutator{
		log:             log.WithValues("component", "DefaultingWebhook"),
		cl:              client,
		dynamicCl:       dynamicClient,
		jsonpathFactory: jsonpathFactory,
	}
}

func (m *DefaultingMutator) InstallInto(srv webhook.Server) {
	srv.Register("/defaults", m)
}

func (m *DefaultingMutator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := admissionv1.AdmissionReview{}
	err := m.readAdmissionReview(r, &review)
	if err != nil {
		m.log.Error(err, "got invalid admission review object")
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}

	if namespaces.IsSystem(review.Request.Namespace) || review.Request.SubResource != "" {
		review.Response = &admissionv1.AdmissionResponse{Allowed: true, UID: review.Request.UID}
		m.respond(review, w)
		return
	}

	log := m.log.WithValues(
		"project_name", review.Request.Namespace,
		"object_name", review.Request.Name,
		"object_namespace", review.Request.Namespace,
		"object_resource", review.Request.Resource.String())

	log.Info("Got AdmissionReview")

	if len(review.Request.Object.Raw) == 0 {
		log.Error(err, "Missing object in AdmissionRequest")
		http.Error(w, "missing object in AdmissionRequest", http.StatusBadRequest)
		return
	}

	response, err := m.applyDefaultsIfNecessary(r.Context(), &review)
	if err != nil {
		m.log.Error(err, "failed to determine defaults for object")
		http.Error(w, "failed to determine defaults for object", http.StatusInternalServerError)
		return
	}

	review.Response = response
	review.Response.UID = review.Request.UID

	m.respond(review, w)
}

func (m *DefaultingMutator) respond(review admissionv1.AdmissionReview, w http.ResponseWriter) {
	respBytes, err := json.Marshal(review)
	if err != nil {
		m.log.Error(err, "failed to encode response")
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		m.log.Error(err, "admission response failed")
	}
}

func (m *DefaultingMutator) applyDefaultsIfNecessary(
	ctx context.Context,
	review *admissionv1.AdmissionReview,
) (*admissionv1.AdmissionResponse, error) {
	req := review.Request
	var obj map[string]any
	if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
		return nil, fmt.Errorf("failed to decode incoming object: %w", err)
	}

	grant, err := m.grantByProjectName(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot read project grants for %s: %w", req.Namespace, err)
	}

	objectGVR := schema.GroupVersionResource{
		Group:    req.Resource.Group,
		Resource: req.Resource.Resource,
	}
	log := m.log.WithValues(
		"project", req.Namespace,
		"resource", req.Resource,
		"object_name", req.Name,
	)

	var patches []jsonPatchOperation
	for _, grantPolicyRef := range grant.Spec.Policies {
		policy, err := m.policyByName(ctx, grantPolicyRef.Name)
		if err != nil {
			log.Error(err, "failed to load ClusterObjectGrantPolicy", "policy_name", grantPolicyRef.Name)
			continue
		}

		if grantPolicyRef.Default == "" && policy.Spec.GrantedResource.Defaults.AnnotationKey == "" {
			// No defaults for this policy
			continue
		}

		for _, ref := range policy.Spec.UsageReferences {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				log.Error(err, "Cannot parse apiVersion field of usage reference",
					"policy_name", grantPolicyRef.Name,
					"field_value", ref.APIVersion,
					"usage_reference", ref,
				)
				continue
			}

			// Versions are not important here
			if gv.Group != objectGVR.Group || ref.Resource != objectGVR.Resource {
				continue
			}

			fieldPath, err := m.jsonpathFactory.Path(ref.FieldPath)
			if err != nil {
				log.Error(err, "Invalid JSONPath expression",
					"expr", ref.FieldPath,
					"policy_name", grantPolicyRef.Name,
				)
			}

			fieldValues := fieldPath.SelectLocated(obj)
			switch {
			case len(fieldValues) == 0:
				log.Info("Skipping non-existing field reference", "expr", ref.FieldPath, "policy_name", grantPolicyRef.Name)
				continue
			case len(fieldValues) > 1:
				log.Info("Skipping ambiguous field reference", "expr", ref.FieldPath, "policy_name", grantPolicyRef)
				continue
			}

			fieldValueNode := fieldValues[0]
			if !m.isFieldNotSet(fieldValueNode.Node) {
				// Field is already set, nothing to do
				continue
			}

			defaultValue, err := m.findDefaultValue(ctx, &grantPolicyRef, policy)
			switch {
			case errors.Is(err, errNoDefault):
				continue
			case errors.Is(err, errMultipleDefaults):
				log.Error(err,
					"Multiple candidates for default object based on annotation lookup,"+
						" will not apply any default based on that",
				)
				continue
			case err != nil:
				log.Error(err, "Cannot find out what is the default")
				continue
			}

			patches = append(patches, jsonPatchOperation{
				Op:    "add",
				Path:  fieldValueNode.Path.Pointer(),
				Value: defaultValue,
			})

			m.log.Info("Injecting default value",
				"object", req.Name,
				"namespace", req.Namespace,
				"policy_name", grantPolicyRef.Name,
				"field_path", ref.FieldPath,
				"default", grantPolicyRef.Default,
			)
		}
	}

	if len(patches) == 0 {
		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patches: %w", err)
	}

	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
	}, nil
}

func (m *DefaultingMutator) isFieldNotSet(v any) bool {
	if v == nil {
		return true
	}
	return v.(string) == ""
}

func (m *DefaultingMutator) jsonPathToJSONPointer(jsonPath string) (string, error) {
	path := jsonPath
	switch {
	case strings.HasPrefix(path, "$."):
		path = path[2:]
	case strings.HasPrefix(path, "$"):
		path = path[1:]
	}

	if path == "" {
		return "", errors.New("root cannot be patched")
	}

	// Escape json pointer special chars, probably will never see those in the input, but just in case.
	parts := strings.Split(path, ".")
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~", "~0")
		p = strings.ReplaceAll(p, "/", "~1")
		parts[i] = p
	}
	path = strings.Join(parts, ".")

	// Now replace [ with "/" and drop ] so that array[0].abc
	// turns into  array/0.abc, and then dot-split and join
	// will give us the correct json pointer of /array/0/abc.
	path = strings.ReplaceAll(path, "[", "/")
	path = strings.ReplaceAll(path, "]", "")

	return "/" + strings.Join(strings.Split(path, "."), "/"), nil
}

type jsonPatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func (m *DefaultingMutator) readAdmissionReview(
	r *http.Request,
	review *admissionv1.AdmissionReview,
) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := errors.New("unexpected Content-Type")
		m.log.Error(
			err,
			"Unexpected Content-Type of admission review request",
			"contentType", contentType,
		)
		return err
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(review)
	if err != nil {
		m.log.Error(err, "Cannot decode AdmissionReview")
		return err
	}
	return nil
}

func (m *DefaultingMutator) grantByProjectName(
	ctx context.Context,
	projectName string,
) (*v1alpha1.ClusterObjectsGrant, error) {
	grant := &v1alpha1.ClusterObjectsGrant{}

	err := m.cl.Get(ctx, client.ObjectKey{Name: projectName}, grant)
	if err != nil {
		m.log.Error(err, "Cannot get grant for project", "project_name", projectName)
		return nil, fmt.Errorf("get grant: %w", err)
	}

	return grant, nil
}

func (m *DefaultingMutator) policyByName(
	ctx context.Context,
	policyName string,
) (*v1alpha1.ClusterObjectGrantPolicy, error) {
	policy := &v1alpha1.ClusterObjectGrantPolicy{}

	err := m.cl.Get(ctx, client.ObjectKey{Name: policyName}, policy)
	if err != nil {
		m.log.Error(err, "Cannot get policy", "policy", policyName)
		return nil, fmt.Errorf("get grant: %w", err)
	}

	return policy, nil
}

var (
	errNoDefault        = errors.New("no default resource provided")
	errMultipleDefaults = errors.New("multiple resources are marked as default")
)

func (m *DefaultingMutator) findDefaultValue(
	ctx context.Context,
	grantPolicyReference *v1alpha1.ApplicablePolicy,
	policy *v1alpha1.ClusterObjectGrantPolicy,
) (string, error) {
	if policy.Spec.GrantedResource.Defaults.AnnotationKey == "" {
		if grantPolicyReference.Default == "" {
			return "", errNoDefault
		}
		return grantPolicyReference.Default, nil
	}

	gv, err := schema.ParseGroupVersion(policy.Spec.GrantedResource.APIVersion)
	if err != nil {
		return "", fmt.Errorf("invalid apiVersion in grantedResource: %w", err)
	}

	gk := schema.GroupKind{
		Group: gv.Group,
		Kind:  policy.Spec.GrantedResource.Kind,
	}

	kindResourceMapping, err := m.cl.RESTMapper().RESTMapping(gk, gv.Version)
	if err != nil {
		return "", fmt.Errorf("map kind to resource: %w", err)
	}

	list, err := m.dynamicCl.Resource(kindResourceMapping.Resource).List(ctx, v1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("search %q for possible default annotations: %w", kindResourceMapping.GroupVersionKind, err)
	}

	annotatedItems := make([]*unstructured.Unstructured, 0, 1)
	for _, item := range list.Items {
		_, hasAnnotation := item.GetAnnotations()[policy.Spec.GrantedResource.Defaults.AnnotationKey]
		if hasAnnotation {
			annotatedItems = append(annotatedItems, &item)
		}
	}

	switch len(annotatedItems) {
	case 0:
		return "", errNoDefault
	case 1:
		return annotatedItems[0].GetName(), nil
	default:
		return "", fmt.Errorf(
			"multiple %+v resources are annotated with %q: %w",
			gk,
			policy.Spec.GrantedResource.Defaults.AnnotationKey,
			errMultipleDefaults,
		)
	}
}
