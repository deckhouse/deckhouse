/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	revokedCMName      = "madison-revoked-project"
	revokedCMNamespace = "d8-monitoring"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/flant-integration/connect_revoke",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "madison_revoke",
			Crontab: "*/5 * * * *",
		},
	},
}, dependency.WithExternalDependencies(revokeHandler))

type madisonResponse struct {
	Error string `json:"error"`
}

type statusRequest struct {
	AuthKey string `json:"auth_key"`
}

func revokeHandler(input *go_hook.HookInput, dc dependency.Container) error {
	licenseKey, ok := input.Values.GetOk(internalLicenseKeyPath)
	if !ok {
		return nil
	}

	// Ignore revoking if madison is disabled.
	cfgMadisonAuthKey := input.ConfigValues.Get(madisonKeyPath).String()
	if cfgMadisonAuthKey == "false" {
		return nil
	}

	madisonAuthKey, ok := input.Values.GetOk(internalMadisonKeyPath)
	if !ok || madisonAuthKey.String() == "false" {
		return nil
	}

	r := statusRequest{AuthKey: madisonAuthKey.String()}
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("cannot marshal status data: %v", r)
	}

	httpCli := dc.GetHTTPClient()
	req, err := http.NewRequest(http.MethodPost, connectStatusURL, bytes.NewReader(payload))
	if err != nil {
		input.LogEntry.Errorf("http request failed: %s", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+licenseKey.String())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpCli.Do(req)
	if err != nil {
		input.LogEntry.Errorf("http call failed: %s", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		return nil // OK
	}

	var madisonResp madisonResponse
	err = json.NewDecoder(resp.Body).Decode(&madisonResp)
	if err != nil {
		input.LogEntry.Errorf("json unmarshaling failed: %s", err)
		return nil // dont this we need an error
	}

	// Create a ConfigMap to indicate revocation and prevent re-registration.
	if madisonResp.Error == "Archived setup" {
		// Create CM to indicate revoked license.
		cm := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      revokedCMName,
				Namespace: revokedCMNamespace,
				Labels: map[string]string{
					"heritage": "flant-integration",
				},
			},
		}
		input.PatchCollector.Create(cm, object_patch.IgnoreIfExists())

		// Remove internal values.
		// No more telemetry right after project become archived.
		input.Values.Remove(internalMadisonKeyPath)
		input.Values.Remove(internalLicenseKeyPath)
	}

	return nil
}
