/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestMakeRequest(t *testing.T) {
	// Create a mock HTTP server for testing
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Check the request URL path
		if r.URL.Path != "/test" {
			t.Errorf("expected URL path /test, got %s", r.URL.Path)
		}

		// Check the request headers
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer TEST_TOKEN" {
			t.Errorf("expected Authorization header value Bearer TEST_TOKEN, got %s", authHeader)
		}

		// Send a mock response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "mock response"}`))
	})

	mockServer := httptest.NewServer(mockHandler)
	defer mockServer.Close()

	// Prepare test data
	requestBody := map[string]string{"key": "value"}
	var responseBody map[string]string

	// Prepare headers
	headers := map[string]string{"Authorization": "Bearer TEST_TOKEN"}

	// Create a new HTTP client
	client := &http.Client{}

	// Make the request
	err := makeRequestWithResponse(client, http.MethodPost, mockServer.URL+"/test", headers, requestBody, &responseBody)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check the response body
	expectedResponseBody := map[string]string{"data": "mock response"}
	if !reflect.DeepEqual(responseBody, expectedResponseBody) {
		t.Errorf("expected response body %v, got %v", expectedResponseBody, responseBody)
	}
}
