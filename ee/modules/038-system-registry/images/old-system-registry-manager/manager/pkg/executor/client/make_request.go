/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type CommonError = string

func makeRequestWithResponse(logger *logrus.Entry, client *http.Client, method, url string, headers map[string]string, requestBody interface{}, responseBody interface{}) error {
	// Marshal request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error marshaling request body: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Log request details
	logger.Tracef("Request url: '%s'", url)
	logger.Tracef("Request body: '%s'", jsonData)

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Log response details
	logger.Tracef("Response body: '%s'", string(body))
	logger.Debugf("%s %s %d", method, url, resp.StatusCode)

	// Check if response body is empty
	if len(body) == 0 {
		return fmt.Errorf("empty response body")
	}

	// Unmarshal successful response
	if err := json.Unmarshal(body, responseBody); err != nil {
		return fmt.Errorf("error unmarshaling response: %w", err)
	}

	return nil
}

func makeRequestWithoutResponse(logger *logrus.Entry, client *http.Client, method, url string, headers map[string]string, requestBody interface{}) error {
	// Marshal request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error marshaling request body: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Log request details
	logger.Tracef("Request url: '%s'", url)
	logger.Tracef("Request body: '%s'", jsonData)

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Log successful response
	logger.Debugf("%s %s %d", method, url, resp.StatusCode)

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
