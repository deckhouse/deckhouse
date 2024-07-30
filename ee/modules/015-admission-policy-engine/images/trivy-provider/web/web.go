/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/open-policy-agent/frameworks/constraint/pkg/externaldata"
)

type scanner interface {
	ScanReport(ctx context.Context, data []byte) externaldata.Response
}

type Handler struct {
	s       scanner
	logger  logr.Logger
	timeout time.Duration
}

func NewHandler(s scanner, timeout time.Duration, logger logr.Logger) *Handler {
	return &Handler{
		s:       s,
		timeout: timeout,
		logger:  logger,
	}
}

func (h *Handler) HandleRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.sendResponse(errHelper("only POST requests are allowed"), w)
			return
		}

		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			h.sendResponse(errHelper(fmt.Sprintf("unable to read request body: %v", err)), w)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		resp := h.s.ScanReport(ctx, requestBody)
		h.sendResponse(resp, w)
	}
}

func (h *Handler) sendResponse(scanResponse externaldata.Response, w http.ResponseWriter) {
	response := externaldata.ProviderResponse{
		APIVersion: "externaldata.gatekeeper.sh/v1alpha1",
		Kind:       "ProviderResponse",
		Response:   scanResponse,
	}
	h.logger.WithValues("response", scanResponse).Info("sending response")

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error(err, "can't write response to gatekeeper")
	}
}

func errHelper(errMsg string) externaldata.Response {
	return externaldata.Response{SystemError: errMsg}
}
