package webhooks

import (
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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var ErrNoPolicyExists = errors.New("no ClusterObjectGrantPolicy exist for resource")

var _ http.Handler = &ClusterObjectGrantValidator{}

type ClusterObjectGrantValidator struct {
	log             logr.Logger
	cl              client.Client
	jsonpathFactory jsonpath.Factory
}

// fieldRule is a single (JSONPath, whitelist) constraint to enforce on the
// incoming object. Several policies may constrain the same field path with
// different whitelists; each rule is evaluated independently so that no
// constraint is lost (unlike a map keyed solely by field path).
type fieldRule struct {
	path      string
	whitelist []string
}

func NewClusterResourceGrantValidator(
	log logr.Logger,
	client client.Client,
	jsonpathFactory jsonpath.Factory,
) *ClusterObjectGrantValidator {
	return &ClusterObjectGrantValidator{
		log:             log.WithValues("component", "ClusterResourceGrantValidator"),
		cl:              client,
		jsonpathFactory: jsonpathFactory,
	}
}

func (v *ClusterObjectGrantValidator) InstallInto(srv webhook.Server) {
	srv.Register("/is-granted", v)
}

func (v *ClusterObjectGrantValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	review := admissionv1.AdmissionReview{}
	err := v.readAdmissionReview(r, &review)
	if err != nil {
		v.log.Error(err, "Invalid AdmissionReview")
		http.Error(w, fmt.Sprintf("invalid AdmissionReview: %v", err), http.StatusInternalServerError)
		return
	}
	if review.Request == nil {
		v.log.Error(nil, "AdmissionReview without request")
		http.Error(w, "AdmissionReview without request", http.StatusBadRequest)
		return
	}

	log := v.log.WithValues(
		"project_name", review.Request.Namespace,
		"object_name", review.Request.Name,
		"object_namespace", review.Request.Namespace,
		"object_resource", review.Request.Resource.String())

	log.Info("Got AdmissionReview")

	// System namespaces and subresources are out of scope.
	if namespaces.IsSystem(review.Request.Namespace) || review.Request.SubResource != "" {
		if err = v.allow(&review, w); err != nil {
			log.Error(err, "Admission response failed")
		}
		return
	}

	if len(review.Request.Object.Raw) == 0 {
		log.Error(nil, "Missing object in AdmissionRequest")
		http.Error(w, "missing object in AdmissionRequest", http.StatusBadRequest)
		return
	}

	grants, err := applicableGrants(r.Context(), v.cl, review.Request.Namespace)
	if err != nil {
		log.Error(err, "Cannot resolve applicable grants")
		http.Error(w, "cannot resolve applicable grants", http.StatusInternalServerError)
		return
	}
	if len(grants) == 0 {
		// No grant applies to this project, nothing to enforce. This must not block
		// admission, especially under failurePolicy: Fail.
		if err = v.allow(&review, w); err != nil {
			log.Error(err, "Admission response failed")
		}
		return
	}

	rules := make([]fieldRule, 0)
	for _, grant := range grants {
		for _, ap := range grant.Spec.Policies {
			policy := &v1alpha1.ClusterObjectGrantPolicy{}
			if err := v.cl.Get(r.Context(), client.ObjectKey{Name: ap.Name}, policy); err != nil {
				if k8serrors.IsNotFound(err) {
					// Grant references a policy that does not exist; skip it.
					continue
				}
				log.Error(err, "Cannot get policy", "policy", ap.Name)
				http.Error(w, "cannot get policy", http.StatusInternalServerError)
				return
			}

			for _, ref := range policy.Spec.UsageReferences {
				gv, err := schema.ParseGroupVersion(ref.APIVersion)
				if err != nil {
					log.Error(err, "Cannot parse apiVersion field",
						"field_value", ref.APIVersion,
						"usage_reference", ref,
					)
					http.Error(w, "cannot parse apiVersion: '"+ref.APIVersion+"'", http.StatusInternalServerError)
					return
				}

				if gv.Group != review.Request.Resource.Group || ref.Resource != review.Request.Resource.Resource {
					continue
				}

				whitelist, err := policyWhitelist(r.Context(), v.cl, ap, policy)
				if err != nil {
					log.Error(err, "Cannot build whitelist", "policy", ap.Name)
					http.Error(w, "cannot build whitelist", http.StatusInternalServerError)
					return
				}
				rules = append(rules, fieldRule{path: ref.FieldPath, whitelist: whitelist})
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

	// On UPDATE, validation is diff-based: a value that already existed in the old
	// object is grandfathered in. This avoids blocking updates to legacy objects that
	// predate the policy (and whose field this update does not change).
	var oldObj map[string]any
	if review.Request.Operation == admissionv1.Update && len(review.Request.OldObject.Raw) > 0 {
		if err := json.Unmarshal(review.Request.OldObject.Raw, &oldObj); err != nil {
			log.Error(err, "Invalid old object")
			http.Error(w, "invalid old object", http.StatusBadRequest)
			return
		}
	}

	for _, rule := range rules {
		parsedPath, err := v.jsonpathFactory.Path(rule.path)
		if err != nil {
			log.Error(err, "invalid JSONPath", "JSONPath", rule.path)
			continue
		}

		fieldValues := parsedPath.Select(obj.Object)
		if len(fieldValues) == 0 {
			log.Info("Skipping non-existing field reference", "expr", rule.path)
			continue
		}

		// Values already present in the old object are exempt (diff-based UPDATE).
		oldValues := make(map[string]struct{})
		if oldObj != nil {
			for _, ov := range parsedPath.Select(oldObj) {
				if s, ok := ov.(string); ok {
					oldValues[s] = struct{}{}
				}
			}
		}

		// Validate every matched value. A JSONPath may legitimately match more
		// than one node (e.g. $.spec.containers[*].image); skipping multi-match
		// results would silently bypass the whitelist.
		for _, fv := range fieldValues {
			s, ok := fv.(string)
			if ok {
				if slices.Contains(rule.whitelist, s) {
					continue
				}
				if _, unchanged := oldValues[s]; unchanged {
					continue
				}
			}
			// Non-string value, or a string that is neither whitelisted nor
			// pre-existing in the old object.
			if err = v.deny(&review, w); err != nil {
				log.Error(err, "admission response failed")
			}
			return
		}
	}

	if err = v.allow(&review, w); err != nil {
		log.Error(err, "admission response failed")
	}
}

func (v *ClusterObjectGrantValidator) readAdmissionReview(
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

func (v *ClusterObjectGrantValidator) allow(
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

func (v *ClusterObjectGrantValidator) deny(
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
