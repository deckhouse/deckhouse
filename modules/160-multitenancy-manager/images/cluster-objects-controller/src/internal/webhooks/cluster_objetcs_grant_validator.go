package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var ErrNoPolicyExists = errors.New("no ClusterObjectGrantPolicy exist for resource")

var _ http.Handler = &ClusterObjectsGrantValidator{}

type ClusterObjectsGrantValidator struct {
	log             logr.Logger
	cl              client.Client
	jsonpathFactory jsonpath.Factory
}

func NewClusterResourceGrantValidator(
	log logr.Logger,
	client client.Client,
	jsonpathFactory jsonpath.Factory,
) *ClusterObjectsGrantValidator {
	return &ClusterObjectsGrantValidator{
		log:             log.WithValues("component", "ClusterResourceGrantValidator"),
		cl:              client,
		jsonpathFactory: jsonpathFactory,
	}
}

func (v *ClusterObjectsGrantValidator) InstallInto(srv webhook.Server) {
	srv.Register("/is-granted", v)
}

func (v *ClusterObjectsGrantValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := admissionv1.AdmissionReview{}
	err := v.readAdmissionReview(r, &review)
	if err != nil {
		v.log.Error(err, "Invalid AdmissionReview")
		http.Error(w, fmt.Sprintf("invalid AdmissionReview: %v", err), http.StatusInternalServerError)
		return
	}

	log := v.log.WithValues(
		"project_name", review.Request.Namespace,
		"object_name", review.Request.Name,
		"object_namespace", review.Request.Namespace,
		"object_resource", review.Request.Resource.String())

	log.Info("Got AdmissionReview")

	if namespaces.IsSystem(review.Request.Namespace) || review.Request.SubResource != "" {
		if err = v.allow(&review, w); err != nil {
			log.Error(err, "Admission response failed")
			return
		}
		return
	}

	if len(review.Request.Object.Raw) == 0 {
		log.Error(err, "Missing object in AdmissionRequest")
		http.Error(w, "missing object in AdmissionRequest", http.StatusBadRequest)
		return
	}

	// Subresources are out of scope.
	if review.Request.SubResource != "" {
		if err = v.allow(&review, w); err != nil {
			log.Error(err, "Admission response failed")
			return
		}
		return
	}

	grant, err := v.grantByProjectName(r.Context(), review.Request.Namespace)
	if err != nil {
		log.Error(err, "Cannot find project grants")
		http.Error(w, "cannot find project grants", http.StatusInternalServerError)
		return
	}
	policies, err := v.policiesForGrant(r.Context(), grant)
	if err != nil {
		log.Error(err, "Cannot list policies")
		http.Error(w, "cannot list policies", http.StatusInternalServerError)
		return
	}
	if len(policies) == 0 {
		// Grant is empty or contains only invalid references to policies, nothing to do
		if err = v.allow(&review, w); err != nil {
			log.Error(err, "Admission response failed")
			return
		}
		return
	}

	fieldsAndValuesToValidate := make(map[string][]string) // JSONPath to value whitelist
	for _, p := range policies {
		for _, ref := range p.Spec.UsageReferences {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				log.Error(err, "Cannot parse apiVersion field",
					"field_value", ref.APIVersion,
					"usage_reference", ref,
				)
				http.Error(w, "cannot parse apiVersion: '"+ref.APIVersion+"'", http.StatusInternalServerError)
				return

			}

			if gv.Group == review.Request.Kind.Group && ref.Resource == review.Request.Resource.Resource {
				for _, g := range grant.Spec.Policies {
					if g.Name == p.Name {
						whitelist := g.Allowed

						// Make sure that default is in whitelist just in case
						if g.Default != "" && !slices.Contains(g.Allowed, g.Default) {
							whitelist = append(whitelist, g.Default)
						}
						fieldsAndValuesToValidate[ref.FieldPath] = whitelist
					}
				}
			}
		}
	}

	obj := &unstructured.Unstructured{Object: make(map[string]any)}
	err = json.Unmarshal(review.Request.Object.Raw, &obj.Object)
	if err != nil {
		log.Error(err, "Invalid object")
		http.Error(w, "invalid object", http.StatusBadRequest)
		return
	}

	for jsonPath, whitelist := range fieldsAndValuesToValidate {
		parsedPath, err := v.jsonpathFactory.Path(jsonPath)
		if err != nil {
			log.Error(err, "invalid JSONPath", "JSONPath", jsonPath)
			continue
		}

		fieldValues := parsedPath.Select(obj.Object)
		switch {
		case len(fieldValues) == 0:
			log.Info("Skipping non-existing field reference", "expr", jsonPath)
			continue
		case len(fieldValues) > 1:
			log.Info("Skipping ambiguous field reference", "expr", jsonPath)
			continue
		}

		if !slices.Contains(whitelist, fieldValues[0].(string)) {
			err = v.deny(&review, w)
			if err != nil {
				log.Error(err, "admission response failed")
				return
			}
			return
		}
	}

	err = v.allow(&review, w)
	if err != nil {
		log.Error(err, "admission response failed")
	}
}

func (v *ClusterObjectsGrantValidator) readAdmissionReview(
	r *http.Request,
	review *admissionv1.AdmissionReview,
) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := errors.New("unexpected Content-Type")
		v.log.Error(
			err,
			"Unexpected Content-Type of admission review request",
			"contentType", contentType,
		)
		return err
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(review)
	if err != nil {
		v.log.Error(err, "Cannot decode AdmissionReview")
		return err
	}
	return nil
}

func (v *ClusterObjectsGrantValidator) policiesForGrant(
	ctx context.Context,
	grant *v1alpha1.ClusterObjectsGrant,
) ([]*v1alpha1.ClusterObjectGrantPolicy, error) {
	if len(grant.Spec.Policies) == 0 {
		return []*v1alpha1.ClusterObjectGrantPolicy{}, nil
	}

	grantPoliciesIndex := make(map[string]struct{})
	for _, p := range grant.Spec.Policies {
		grantPoliciesIndex[p.Name] = struct{}{}
	}

	policiesList := &v1alpha1.ClusterObjectGrantPolicyList{}
	v.cl.List(ctx, policiesList)

	grantPolicies := make([]*v1alpha1.ClusterObjectGrantPolicy, 0)
	for _, policy := range policiesList.Items {
		_, found := grantPoliciesIndex[policy.Name]
		if !found {
			continue
		}

		grantPolicies = append(grantPolicies, policy.DeepCopy())
	}

	return grantPolicies, nil
}

func (v *ClusterObjectsGrantValidator) grantByProjectName(
	ctx context.Context,
	projectName string,
) (*v1alpha1.ClusterObjectsGrant, error) {
	grant := &v1alpha1.ClusterObjectsGrant{}

	err := v.cl.Get(ctx, client.ObjectKey{Name: projectName}, grant)
	if err != nil {
		v.log.Error(err, "Cannot get grant for project", "project_name", projectName)
		return nil, fmt.Errorf("get grant: %w", err)
	}

	return grant, nil
}

func (v *ClusterObjectsGrantValidator) allow(
	review *admissionv1.AdmissionReview,
	w http.ResponseWriter,
) error {
	v.log.Info("Allowed resource",
		"resource", review.Request.Resource,
		"name", review.Request.Name,
		"namespace", review.Request.Namespace,
	)

	review.Response = &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(review)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	return nil
}

func (v *ClusterObjectsGrantValidator) deny(
	review *admissionv1.AdmissionReview,
	w http.ResponseWriter,
) error {
	v.log.Info("Denied resource",
		"resource", review.Request.Resource,
		"name", review.Request.Name,
		"namespace", review.Request.Namespace,
	)
	review.Response = &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: false,
		Result: &metav1.Status{
			Message: fmt.Sprintf(
				"[cluster-objects-controller] Use of %s/%s is not allowed for project %s. "+
					"Contact cluster operators to enable its usage.",
				review.Request.Kind, review.Request.Name, review.Request.Namespace,
			),
			Code: http.StatusForbidden,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(review)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	return nil
}
