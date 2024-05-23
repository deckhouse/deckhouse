/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"encoding/json"
	"net/http"
)

const (
	MasterInfoUrlPattern     = "/master_info"
	CheckRegistryUrlPattern  = "/check_registry"
	UpdateRegistryUrlPattern = "/update_registry"
	DeleteRegistryUrlPattern = "/delete_registry"
	IsBusyUrlPattern         = "/is_busy"
)

func RequestMasterInfo(client *http.Client, url string, headers map[string]string, response *MasterInfoResponse) error {
	return makeRequestWithResponse(client, "POST", url, headers, nil, response)
}

func RequestCheckRegistry(client *http.Client, url string, headers map[string]string, request *CheckRegistryRequest, response *CheckRegistryResponse) error {
	return makeRequestWithResponse(client, "POST", url, headers, request, response)
}

func RequestUpdateRegistry(client *http.Client, url string, headers map[string]string, request *UpdateRegistryRequest) error {
	return makeRequestWithoutResponse(client, "POST", url, headers, request)
}

func RequestDeleteRegistry(client *http.Client, url string, headers map[string]string) error {
	return makeRequestWithoutResponse(client, "POST", url, headers, nil)
}

func CreateMasterInfoHandlerFunc(f func() (*MasterInfoResponse, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		masterInfo, err := f()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		jsonResponse, err := json.Marshal(masterInfo)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		checkRegistryResponse, err := f(&requestBody)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		jsonResponse, err := json.Marshal(checkRegistryResponse)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		err = f(&requestBody)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
	return SingleRequestMiddlewares(http.HandlerFunc(handlerFunc), cfg)
}

func CreateIsBusyHandlerFunc(cfg *SingleRequestConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var requestBody CheckRegistryRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		responce := BusyResponce{
			Data: struct {
				IsBusy bool "json:\"isBusy\""
			}{
				IsBusy: cfg.IsBusy(),
			},
		}

		jsonResponse, err := json.Marshal(responce)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}
