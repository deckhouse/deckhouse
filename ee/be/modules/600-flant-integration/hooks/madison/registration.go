/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// This hooks registers cluster in madison using license key and store an authentication token in the Secret.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/flant-integration/connect_registration",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       madisonSecretBinding,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{madisonSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{madisonSecretNS},
				},
			},
			// Synchronization is redundant because of OnBeforeHelm.
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			ExecuteHookOnEvents:          go_hook.Bool(false),
			FilterFunc:                   filterMadisonSecret,
		},
		{
			Name:       prometheusSecretBinding,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{prometheusSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{prometheusSecretNS},
				},
			},
			// Synchronization is redundant because of OnBeforeHelm.
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			ExecuteHookOnEvents:          go_hook.Bool(false),
			FilterFunc:                   filterPrometheusSecret,
		},
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, dependency.WithExternalDependencies(registrationHandler))

const (
	connectBaseURL   = "https://connect.deckhouse.io"
	registrationURL  = connectBaseURL + "/v1/madison_register"
	connectStatusURL = connectBaseURL + "/v1/madison_status"

	madisonKeyPath         = "flantIntegration.madisonAuthKey"
	internalMadisonKeyPath = "flantIntegration.internal.madisonAuthKey"
	internalLicenseKeyPath = "flantIntegration.internal.licenseKey"

	prometheusSecretNS      = "d8-monitoring"
	prometheusSecretName    = "prometheus-url-schema"
	prometheusSecretField   = "url_schema"
	prometheusSecretBinding = prometheusSecretName

	madisonSecretNS      = "d8-monitoring"
	madisonSecretName    = "madison-proxy"
	madisonSecretField   = "auth-key"
	madisonSecretBinding = madisonSecretName
)

func filterMadisonSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	return string(secret.Data[madisonSecretField]), nil
}

func filterPrometheusSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	return string(secret.Data[prometheusSecretField]), nil
}

func registrationHandler(input *go_hook.HookInput, dc dependency.Container) error {
	// Remove madisonAuthKey if license is not set.
	licenseKey, ok := input.Values.GetOk(internalLicenseKeyPath)
	if !ok {
		input.Values.Remove(internalMadisonKeyPath)
		return nil
	}

	// Support existing clusters: restore auth key from configuration.
	madisonKey, ok := input.ConfigValues.GetOk(madisonKeyPath)
	if ok {
		input.Values.Set(internalMadisonKeyPath, madisonKey.String())
		return nil
	}

	// Restore auth key from the Secret. (e.g. after restart).
	if len(input.Snapshots[madisonSecretBinding]) > 0 {
		storedAuthKey := input.Snapshots[madisonSecretBinding][0].(string)
		input.Values.Set(internalMadisonKeyPath, storedAuthKey)
		return nil
	}

	// Return if auth key is already in values.
	_, ok = input.Values.GetOk(internalMadisonKeyPath)
	if ok {
		return nil
	}

	// No auth key set in configuration, no auth key stored in the Secret — register in madison.
	domainTemplate := input.Values.Get("global.modules.publicDomainTemplate").String()
	prometheusURLSchema := getPrometheusURLSchema(input)

	// Create payload for Madison with Prometheus and Grafana URLs.
	// Use https mode calculated in 300-prometheus module.
	payload := createMadisonPayload(domainTemplate, prometheusURLSchema)

	// Create http request to d8-connect proxy.
	req, err := newRegistrationRequest(registrationURL, payload, licenseKey.String())
	if err != nil {
		input.LogEntry.Errorf("http request failed: %v", err)
		return nil
	}

	// Make request to madison API.
	authKey, err := doMadisonRequest(req, dc, input.LogEntry)
	if err != nil {
		err := fmt.Errorf("cannot register in madison (%s %s): %v", req.Method, req.URL, err)
		input.LogEntry.Errorf(err.Error())
		return err
	}

	// Save new auth key to Secret and put it to values.
	if authKey != "" {
		input.Values.Set(internalMadisonKeyPath, authKey)
	}

	return nil
}

type madisonRequestData struct {
	Type          string    `json:"type,omitempty"`
	Name          string    `json:"name"`
	PrometheusURL string    `json:"prometheus_url"`
	GrafanaURL    string    `json:"grafana_url"`
	ExtraData     extraData `json:"extra_data"`
}

type extraData struct {
	Labels map[string]string `json:"labels"`
}

func createMadisonPayload(domainTemplate string, schema string) madisonRequestData {
	data := madisonRequestData{
		PrometheusURL: "-",
		GrafanaURL:    "-",
	}
	if domainTemplate == "" {
		return data
	}

	data.GrafanaURL = schema + "://" + fmt.Sprintf(domainTemplate, "grafana")
	data.PrometheusURL = data.GrafanaURL + "/prometheus"
	data.Type = "prometheus"

	return data
}

// getPrometheusURLSchema returns the Prometheus module url schema from Secret.
func getPrometheusURLSchema(input *go_hook.HookInput) string {
	snap := input.Snapshots[prometheusSecretBinding]
	if len(snap) == 0 {
		return "http"
	}

	return snap[0].(string)
}

type madisonAuthKeyResp struct {
	Error   string `json:"error"`
	AuthKey string `json:"auth_key"`
}

// doMadisonRequest makes auth request and expect response in form of Madison API
func doMadisonRequest(req *http.Request, dc dependency.Container, logEntry *logrus.Entry) (string, error) {
	resp, err := dc.GetHTTPClient().Do(req)
	if err != nil {
		logEntry.Errorf("http call failed: %s", err)
		return "", nil
	}
	defer resp.Body.Close()

	var madisonResp madisonAuthKeyResp
	err = json.NewDecoder(resp.Body).Decode(&madisonResp)
	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		logEntry.Errorf("json unmarshaling failed(body: %q): %v", string(body), err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("%d %s", resp.StatusCode, resp.Status)
		if madisonResp.Error != "" {
			errMsg += ": " + madisonResp.Error
		}
		return "", errors.New(errMsg)
	}

	return madisonResp.AuthKey, nil
}

type registrationData struct {
	Payload string `json:"madisonData"`
}

func newRegistrationRequest(endpoint string, data madisonRequestData, key string) (*http.Request, error) {
	madisonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal madison request data")
	}
	proxyData := registrationData{
		Payload: string(madisonData),
	}
	proxyPayload, err := json.Marshal(proxyData)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal d8-connect request data")
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(proxyPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}
