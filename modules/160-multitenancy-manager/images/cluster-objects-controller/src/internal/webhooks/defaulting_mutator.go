package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"controller/api/v1alpha1"
	"controller/internal/namespaces"

	"github.com/PaesslerAG/jsonpath"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ http.Handler = &DefaultingMutator{}

type DefaultingMutator struct {
	log logr.Logger
	cl  client.Client
}

func NewDefaultingMutator(
	log logr.Logger,
	client client.Client,
) *DefaultingMutator {
	return &DefaultingMutator{
		log: log,
		cl:  client,
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

	var patches []jsonPatchOperation
	for _, grantPolicyRef := range grant.Spec.Policies {
		if grantPolicyRef.Default == "" {
			// No default for this policy
			continue
		}

		policy, err := m.policyByName(ctx, grantPolicyRef.Name)
		if err != nil {
			m.log.Error(err, "failed to load ClusterObjectGrantPolicy", "policy_name", grantPolicyRef.Name)
			continue
		}

		for _, ref := range policy.Spec.UsageReferences {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				m.log.Error(err, "Cannot parse apiVersion field of usage reference",
					"policy_name", grantPolicyRef.Name,
					"field_value", ref.APIVersion,
					"usage_reference", ref,
				)
				continue
			}

			refGVR := schema.GroupVersionResource{
				Group:    gv.Group,
				Resource: ref.Resource,
			}

			// Versions are not important here
			if refGVR.Group != objectGVR.Group || refGVR.Resource != objectGVR.Resource {
				continue
			}

			fieldValue, err := jsonpath.Get(ref.FieldPath, obj)
			if err != nil {
				m.log.Error(err, "failed to evaluate fieldPath", "field_path", ref.FieldPath, "policy_name", grantPolicyRef.Name)
				continue
			}

			if !m.isFieldNotSet(fieldValue) {
				// Field is already set, nothing to do
				continue
			}

			pointer, err := m.jsonPathToJSONPointer(ref.FieldPath)
			if err != nil {
				m.log.Error(err, "failed to convert fieldPath to JSON Pointer", "field_path", ref.FieldPath)
				continue
			}

			patches = append(patches, jsonPatchOperation{
				Op:    "add",
				Path:  pointer,
				Value: grantPolicyRef.Default,
			})

			m.log.Info("injecting default value",
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
