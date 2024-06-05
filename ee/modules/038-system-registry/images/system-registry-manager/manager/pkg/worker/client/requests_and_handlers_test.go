/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMasterInfoRequests(t *testing.T) {
	// Expected master info response
	mockResponse := MasterInfoResponse{
		Data: struct {
			IsMaster          bool   `json:"isMaster"`
			MasterName        string `json:"masterName"`
			CurrentMasterName string `json:"currentMasterName"`
		}{
			IsMaster:          true,
			MasterName:        "TestMaster",
			CurrentMasterName: "OtherTestMaster",
		},
	}

	// Mock function for fetching master info
	mockMasterInfoFunc := func() (*MasterInfoResponse, error) {
		return &mockResponse, nil
	}

	// Create test handler function using the mockMasterInfoFunc
	handlerFunc := CreateMasterInfoHandlerFunc(mockMasterInfoFunc)

	// Create a mock HTTP server using the handler function
	mockServer := httptest.NewServer(http.HandlerFunc(handlerFunc))
	defer mockServer.Close()

	// Make a request to the mock server
	var response MasterInfoResponse
	err := RequestMasterInfo(logrus.NewEntry(logrus.New()), &http.Client{}, mockServer.URL, map[string]string{}, &response)
	assert.NoError(t, err)

	// Compare received master info with expected
	if !reflect.DeepEqual(mockResponse, response) {
		t.Errorf("expected response body %v, got %v", mockResponse, response)
	}
}

func TestCheckRegistryRequests(t *testing.T) {
	// Expected check registry response
	mockResponse := CheckRegistryResponse{
		Data: struct {
			RegistryFilesState struct {
				ManifestsWaitToCreate    bool `json:"manifestsWaitToCreate"`
				ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
				StaticPodsWaitToCreate   bool `json:"staticPodsWaitToCreate"`
				StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
				CertificatesWaitToCreate bool `json:"certificatesWaitToCreate"`
				CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
			} `json:"registryState"`
		}{
			RegistryFilesState: struct {
				ManifestsWaitToCreate    bool `json:"manifestsWaitToCreate"`
				ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
				StaticPodsWaitToCreate   bool `json:"staticPodsWaitToCreate"`
				StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
				CertificatesWaitToCreate bool `json:"certificatesWaitToCreate"`
				CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
			}{
				ManifestsWaitToCreate:    true,
				ManifestsWaitToUpdate:    false,
				StaticPodsWaitToCreate:   true,
				StaticPodsWaitToUpdate:   false,
				CertificatesWaitToCreate: true,
				CertificatesWaitToUpdate: false,
			},
		},
	}

	mockRequest := CheckRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string `json:"masterPeers"`
		}{
			MasterPeers: []string{"123", "123", "321"},
		},
	}

	// Mock function for checking registry
	mockCheckRegistryFunc := func(request *CheckRegistryRequest) (*CheckRegistryResponse, error) {
		if !reflect.DeepEqual(&mockRequest, request) {
			t.Errorf("expected request body %v, got %v", mockRequest, request)
		}
		return &mockResponse, nil
	}

	// Create test handler function using the mockCheckRegistryFunc
	singleRequestCfg := CreateSingleRequestConfig()
	handler := CreateCheckRegistryHandler(mockCheckRegistryFunc, singleRequestCfg)

	// Create a mock HTTP server using the handler
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	// Make a request to the mock server
	var response CheckRegistryResponse
	err := RequestCheckRegistry(logrus.NewEntry(logrus.New()), &http.Client{}, mockServer.URL, map[string]string{}, &mockRequest, &response)
	assert.NoError(t, err)

	// Compare received response with expected
	if !reflect.DeepEqual(mockResponse, response) {
		t.Errorf("expected response body %v, got %v", mockResponse, response)
	}
}

func TestUpdateRegistryRequests(t *testing.T) {
	// Expected update registry request
	mockRequest := UpdateRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string `json:"masterPeers"`
		}{
			MasterPeers: []string{"123", "123", "321"},
		},
	}

	// Mock function for updating registry
	mockUpdateRegistryFunc := func(request *UpdateRegistryRequest) error {
		if !reflect.DeepEqual(&mockRequest, request) {
			t.Errorf("expected request body %v, got %v", mockRequest, request)
		}
		return nil
	}

	// Create test handler function using the mockUpdateRegistryFunc
	singleRequestCfg := CreateSingleRequestConfig()
	handler := CreateUpdateRegistryHandler(mockUpdateRegistryFunc, singleRequestCfg)

	// Create a mock HTTP server using the handler
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	// Make a request to the mock server
	err := RequestUpdateRegistry(logrus.NewEntry(logrus.New()), &http.Client{}, mockServer.URL, map[string]string{}, &mockRequest)
	assert.NoError(t, err)
}

func TestDeleteRegistryRequests(t *testing.T) {
	// Mock function for deleting registry
	mockDeleteRegistryFunc := func() error {
		return nil
	}

	// Create test handler function using the mockDeleteRegistryFunc
	singleRequestCfg := CreateSingleRequestConfig()
	handler := CreateDeleteRegistryHandler(mockDeleteRegistryFunc, singleRequestCfg)

	// Create a mock HTTP server using the handler
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	// Make a request to the mock server
	err := RequestDeleteRegistry(logrus.NewEntry(logrus.New()), &http.Client{}, mockServer.URL, map[string]string{})
	assert.NoError(t, err)
}

func TestIsBusyRequests(t *testing.T) {
	// Expected check registry response
	mockResponse := IsBusyResponse{
		Data: struct {
			IsBusy bool `json:"isBusy"`
		}{
			IsBusy: false,
		},
	}

	mockRequest := IsBusyRequest{
		WaitTimeoutSeconds: nil,
	}

	// Create test handler function using the mockCheckRegistryFunc
	singleRequestCfg := CreateSingleRequestConfig()
	handler := CreateIsBusyHandlerFunc(singleRequestCfg)

	// Create a mock HTTP server using the handler
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	// Make a request to the mock server
	var response IsBusyResponse
	err := RequestIsBusy(logrus.NewEntry(logrus.New()), &http.Client{}, mockServer.URL, map[string]string{}, &mockRequest, &response)
	assert.NoError(t, err)

	// Compare received response with expected
	if !reflect.DeepEqual(mockResponse, response) {
		t.Errorf("expected response body %v, got %v", mockResponse, response)
	}
}
