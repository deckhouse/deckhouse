/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"report-updater/vulndb"

	v1alpha1 "github.com/aquasecurity/trivy-operator/pkg/apis/aquasecurity/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
)

var _ http.Handler = (*Handler)(nil)

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger *log.Logger
	bdu    vulndb.Cache
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewHandler(logger *log.Logger, bdu vulndb.Cache) (*Handler, error) {
	return &Handler{
		logger: logger,
		bdu:    bdu,
	}, nil
}

func (h *Handler) StartRenewBduCache(ch chan struct{}) error {
	ch <- struct{}{}
	return nil
}

func (h *Handler) CheckBDU() error {
	return h.bdu.Check()
}

func (h *Handler) createPatch(req *admissionv1.AdmissionReview) ([]patchOperation, error) {
	var patches []patchOperation
	var report v1alpha1.VulnerabilityReport

	err := json.Unmarshal(req.Request.Object.Raw, &report)
	if err != nil {
		return nil, err
	}

	for k, v := range report.Report.Vulnerabilities {
		if !strings.HasPrefix(v.VulnerabilityID, "BDU") {
			entry, found := h.bdu.Get(v.VulnerabilityID)
			if found && len(entry.IDs) > 0 {
				patches = append(patches, patchOperation{
					Op:    "replace",
					Path:  fmt.Sprintf("/report/vulnerabilities/%d/vulnerabilityID", k),
					Value: strings.Replace(entry.IDs[0], "BDU:", "BDU-", 1),
				})
			}
		}
	}

	return patches, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		return
	}

	admissionReviewReq := &admissionv1.AdmissionReview{}
	var err error
	if err := json.NewDecoder(r.Body).Decode(admissionReviewReq); err != nil {
		// this case is exceptional
		h.logger.Fatalf("cannot unmarshal kubernetes request: %v", err)
	}

	if admissionReviewReq, err = h.mutateRequest(admissionReviewReq); err != nil {
		h.logger.Fatalf("cannot mutate request: %v", err)
	}

	respData, err := json.Marshal(admissionReviewReq)
	if err != nil {
		// this case is exceptional
		h.logger.Fatalf("cannot marshal json response: %v", respData)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respData)
}

func (h *Handler) mutateRequest(review *admissionv1.AdmissionReview) (*admissionv1.AdmissionReview, error) {
	patches, err := h.createPatch(review)

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, err
	}

	patchType := admissionv1.PatchTypeJSONPatch

	h.logger.Println("mutate request", review.Request.Name)

	var admissionReviewResponse = &admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			UID:       review.Request.UID,
			Allowed:   true,
			Patch:     patchBytes,
			PatchType: &patchType,
		},
	}

	admissionReviewResponse.Kind = "AdmissionReview"
	admissionReviewResponse.APIVersion = "admission.k8s.io/v1"

	return admissionReviewResponse, nil
}
