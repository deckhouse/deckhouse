/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	envDynamixAppID         = "DYNAMIX_APP_ID"
	envDynamixAppSecret     = "DYNAMIX_APP_SECRET"
	envDynamixOAuth2URL     = "DYNAMIX_OAUTH2_URL"
	envDynamixControllerURL = "DYNAMIX_CONTROLLER_URL"
	envDynamixInsecure      = "DYNAMIX_INSECURE"
)

func NewCredentials() (*Credentials, error) {
	credentialsConfig := &Credentials{}
	appID := os.Getenv(envDynamixAppID)
	if appID == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixAppID)
	}
	credentialsConfig.AppID = appID

	appSecret := os.Getenv(envDynamixAppSecret)
	if appSecret == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixAppSecret)
	}
	credentialsConfig.AppSecret = appSecret

	oAuth2URL := os.Getenv(envDynamixOAuth2URL)
	if oAuth2URL == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixOAuth2URL)
	}
	credentialsConfig.OAuth2URL = oAuth2URL

	controllerURL := os.Getenv(envDynamixControllerURL)
	if controllerURL == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixControllerURL)
	}
	credentialsConfig.ControllerURL = controllerURL

	credentialsConfig.Insecure = strings.ToLower(os.Getenv(envDynamixInsecure)) == "true"
	return credentialsConfig, nil
}
