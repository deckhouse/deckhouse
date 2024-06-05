/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
)

const (
	MasterInfoUrlPattern     = "/master_info"
	CheckRegistryUrlPattern  = "/check_registry"
	UpdateRegistryUrlPattern = "/update_registry"
	DeleteRegistryUrlPattern = "/delete_registry"
	IsBusyUrlPattern         = "/is_busy"
)

func RequestMasterInfo(logger *logrus.Entry, client *http.Client, url string, headers map[string]string, response *MasterInfoResponse) error {
	return makeRequestWithResponse(logger, client, http.MethodPost, url+MasterInfoUrlPattern, headers, nil, response)
}

func RequestCheckRegistry(logger *logrus.Entry, client *http.Client, url string, headers map[string]string, request *CheckRegistryRequest, response *CheckRegistryResponse) error {
	return makeRequestWithResponse(logger, client, http.MethodPost, url+CheckRegistryUrlPattern, headers, request, response)
}

func RequestUpdateRegistry(logger *logrus.Entry, client *http.Client, url string, headers map[string]string, request *UpdateRegistryRequest) error {
	return makeRequestWithoutResponse(logger, client, http.MethodPost, url+UpdateRegistryUrlPattern, headers, request)
}

func RequestDeleteRegistry(logger *logrus.Entry, client *http.Client, url string, headers map[string]string) error {
	return makeRequestWithoutResponse(logger, client, http.MethodPost, url+DeleteRegistryUrlPattern, headers, nil)
}

func RequestIsBusy(logger *logrus.Entry, client *http.Client, url string, headers map[string]string, request *IsBusyRequest, response *IsBusyResponse) error {
	return makeRequestWithResponse(logger, client, http.MethodPost, url+IsBusyUrlPattern, headers, request, response)
}

func CreateMasterInfoHandlerFunc(f func() (*MasterInfoResponse, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		masterInfo, err := f()
		if err != nil {
			http.Error(w, "Failed to retrieve master info", http.StatusInternalServerError)
			return
		}

		jsonResponse, err := json.Marshal(masterInfo)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}

func CreateCheckRegistryHandler(f func(*CheckRegistryRequest) (*CheckRegistryResponse, error), cfg *SingleRequestConfig) http.Handler {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var requestBody CheckRegistryRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusInternalServerError)
			return
		}

		checkRegistryResponse, err := f(&requestBody)
		if err != nil {
			http.Error(w, "Failed to process check registry request", http.StatusInternalServerError)
			return
		}

		jsonResponse, err := json.Marshal(checkRegistryResponse)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
	return SingleRequestMiddlewares(http.HandlerFunc(handlerFunc), cfg)
}

func CreateUpdateRegistryHandler(f func(*UpdateRegistryRequest) error, cfg *SingleRequestConfig) http.Handler {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var requestBody UpdateRegistryRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusInternalServerError)
			return
		}

		err = f(&requestBody)
		if err != nil {
			http.Error(w, "Failed to process update registry request", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
	return SingleRequestMiddlewares(http.HandlerFunc(handlerFunc), cfg)
}

func CreateDeleteRegistryHandler(f func() error, cfg *SingleRequestConfig) http.Handler {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := f()
		if err != nil {
			http.Error(w, "Failed to process delete registry request", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
	return SingleRequestMiddlewares(http.HandlerFunc(handlerFunc), cfg)
}

func CreateIsBusyHandlerFunc(cfg *SingleRequestConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var requestBody IsBusyRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusInternalServerError)
			return
		}

		waitTimeoutSeconds := 0
		if requestBody.WaitTimeoutSeconds != nil {
			waitTimeoutSeconds = *requestBody.WaitTimeoutSeconds
		}

		response := IsBusyResponse{
			Data: struct {
				IsBusy bool `json:"isBusy"`
			}{
				IsBusy: cfg.WaitIsBusy(waitTimeoutSeconds),
			},
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}
