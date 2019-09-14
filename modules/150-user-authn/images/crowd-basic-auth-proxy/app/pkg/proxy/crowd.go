package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func makeCrowdRequest(h handler, method, apiUrl string, jsonPayload interface{}) (string, error) {
	var body io.Reader
	if jsonPayload != nil {
		jsonData, err := json.Marshal(jsonPayload)
		if err != nil {
			return "", fmt.Errorf("crowd request error: %+v", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s/rest/usermanagement/1/%s", h.CrowdBaseUrl, apiUrl), body)
	if err != nil {
		return "", fmt.Errorf("crowd request error: %+v", err)
	}

	req.SetBasicAuth(h.ClientID, h.ClientSecret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	tr := &http.Transport{
		// todo: ensure that we really need InsecureSkipVerify here
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("crowd request error: %+v", err)
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
		return "", fmt.Errorf("crowd request was not successful: %v %v", resp.StatusCode, string(responseBody))
	}
	return string(responseBody), nil
}

func getCrowdGroups(body string) ([]string, error) {
	var crowdGroups struct {
		Groups []struct{ Name string } `json:"groups"`
	}
	var groups []string

	if err := json.Unmarshal([]byte(body), &crowdGroups); err != nil {
		return groups, err
	}
	for _, value := range crowdGroups.Groups {
		groups = append(groups, value.Name)
	}
	return groups, nil
}
