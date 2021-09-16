/*
Copyright 2021 Flant CJSC
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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
	madisonAuthKey, ok := input.Values.GetOk(madisonKeyPath)
	if !ok {
		return nil
	}

	if madisonAuthKey.String() == "false" {
		return nil
	}

	licenseKey, ok := input.Values.GetOk(licenseKeyPath)
	if !ok {
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

	if madisonResp.Error == "Archived setup" {
		input.ConfigValues.Remove("flantIntegration.licenseKey")
		input.ConfigValues.Remove("flantIntegration.madisonAuthKey")
	}

	return nil
}
