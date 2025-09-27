/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"report-updater/internal/cache"

	admissionv1 "k8s.io/api/admission/v1"
)

var _ http.Handler = (*Handler)(nil)

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger   *log.Logger
	dict     *cache.VulnerabilityCache
	settings *HandlerSettings
}

type HandlerSettings struct {
	DictRenewInterval time.Duration
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewHandler(logger *log.Logger, dict *cache.VulnerabilityCache, settings *HandlerSettings) (*Handler, error) {
	return &Handler{
		logger:   logger,
		dict:     dict,
		settings: settings,
	}, nil
}

func (h *Handler) StartRenewCacheLoop(ctx context.Context) {
	ticker := time.NewTicker(h.settings.DictRenewInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.logger.Println("Starting periodic dict update")
		h.dict.Renew(ctx)
	}
}

func (h *Handler) CheckBDU() error {
	return h.dict.Check()
}

func (h *Handler) createPatch(req *admissionv1.AdmissionReview) ([]patchOperation, error) {
	var patches []patchOperation
	var report VulnerabilityReport

	if err := json.Unmarshal(req.Request.Object.Raw, &report); err != nil {
		return nil, err
	}

	for k, v := range report.Report.Vulnerabilities {
		if !strings.HasPrefix(v.VulnerabilityID, "BDU") {
			entry, found := h.dict.Get(v.VulnerabilityID)
			if found && len(entry) > 0 {
				patches = append(patches, patchOperation{
					Op:    "replace",
					Path:  fmt.Sprintf("/report/vulnerabilities/%d/vulnerabilityID", k),
					Value: strings.Replace(entry[0], "BDU:", "BDU-", 1),
				})
			} else {
				h.logger.Printf("BDU match not found for %s\n", v.VulnerabilityID)
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

	admissionReviewReq := new(admissionv1.AdmissionReview)
	if err := json.NewDecoder(r.Body).Decode(admissionReviewReq); err != nil {
		// this case is exceptional
		h.logger.Fatalf("cannot unmarshal kubernetes request: %v", err)
		return
	}

	var err error
	if admissionReviewReq, err = h.mutateRequest(admissionReviewReq); err != nil {
		h.logger.Fatalf("cannot mutate report: %s: %v", admissionReviewReq.Request.Name, err)
		return
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
	if err != nil {
		return nil, err
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, err
	}

	patchType := admissionv1.PatchTypeJSONPatch

	h.logger.Println("mutate report", review.Request.Name)

	admissionReviewResponse := &admissionv1.AdmissionReview{
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
