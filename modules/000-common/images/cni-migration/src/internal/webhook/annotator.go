/*
Copyright 2025 Flant JSC

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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cnimigrationv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
)

const (
	EffectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
)

type PodAnnotator struct {
	Client client.Client
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (a *PodAnnotator) Handle(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context())

	admissionReview, err := decodeAdmissionReview(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not decode admission review: %v", err), http.StatusBadRequest)
		return
	}

	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: true,
	}

	pod, err := decodePod(admissionReview)
	if err != nil {
		sendAdmissionResponse(w, admissionResponse)
		return
	}

	if pod.Spec.HostNetwork {
		sendAdmissionResponse(w, admissionResponse)
		return
	}

	currentCNI, err := a.getCurrentCNI(r.Context())
	if err != nil || currentCNI == "" {
		// If we can't determine the CNI, we allow the pod creation without annotation.
		sendAdmissionResponse(w, admissionResponse)
		return
	}

	patch, err := createPatch(pod, currentCNI)
	if err != nil {
		// Again, fail open.
		sendAdmissionResponse(w, admissionResponse)
		return
	}

	if patch != nil {
		patchType := admissionv1.PatchTypeJSONPatch
		admissionResponse.Patch = patch
		admissionResponse.PatchType = &patchType
		logger.Info(
			"Pod annotated with effective CNI", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name), "CNI", currentCNI,
		)
	}

	sendAdmissionResponse(w, admissionResponse)
}

func (a *PodAnnotator) getCurrentCNI(ctx context.Context) (string, error) {
	migrations := &cnimigrationv1alpha1.CNIMigrationList{}
	err := a.Client.List(ctx, migrations, &client.ListOptions{
		LabelSelector: labels.Everything(),
	})
	if err != nil {
		return "", err
	}

	// Find the migration that is not in a final state
	for _, migration := range migrations.Items {
		isSucceeded := false
		for _, cond := range migration.Status.Conditions {
			if cond.Type == "Succeeded" && cond.Status == metav1.ConditionTrue {
				isSucceeded = true
				break
			}
		}

		if !isSucceeded {
			return migration.Status.CurrentCNI, nil
		}
	}

	return "", nil // No active migration found
}

func createPatch(pod *corev1.Pod, currentCNI string) ([]byte, error) {
	var patches []patchOperation

	annotations := pod.Annotations
	if annotations == nil {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: map[string]string{EffectiveCNIAnnotation: currentCNI},
		})
	} else if annotations[EffectiveCNIAnnotation] == "" {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/annotations/%s", escapeJSONPointer(EffectiveCNIAnnotation)),
			Value: currentCNI,
		})
	}

	if len(patches) == 0 {
		return nil, nil
	}

	return json.Marshal(patches)
}

func decodeAdmissionReview(r *http.Request) (*admissionv1.AdmissionReview, error) {
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		return nil, err
	}
	if admissionReview.Request == nil {
		return nil, fmt.Errorf("admission review request is nil")
	}
	return &admissionReview, nil
}

func decodePod(review *admissionv1.AdmissionReview) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return nil, fmt.Errorf("could not unmarshal pod: %v", err)
	}
	return &pod, nil
}

func sendAdmissionResponse(w http.ResponseWriter, resp *admissionv1.AdmissionResponse) {
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: resp,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(review); err != nil {
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
}

// escapeJSONPointer escapes characters that have special meaning in a JSON Pointer.
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
