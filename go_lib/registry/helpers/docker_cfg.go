/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type authConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

type dockerConfig struct {
	Auths map[string]authConfig `json:"auths"`
}

func DockerCfgFromCreds(username, password, host string) ([]byte, error) {
	host, err := normalizeHost(host)
	if err != nil {
		return nil, fmt.Errorf("normalize host: %w", err)
	}

	auth := authConfig{
		Username: username,
		Password: password,
	}
	if username != "" && password != "" {
		auth.Auth = encodeAuth(username, password)
	}

	config := dockerConfig{
		Auths: map[string]authConfig{
			host: auth,
		},
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal docker config: %w", err)
	}
	return configJSON, nil
}

func CredsFromDockerCfg(rawConfig []byte, host string) (string, string, error) {
	if len(rawConfig) == 0 {
		return "", "", nil
	}

	var config dockerConfig
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		return "", "", fmt.Errorf("unmarshal docker config: %w", err)
	}

	host, err := normalizeHost(host)
	if err != nil {
		return "", "", fmt.Errorf("normalize host: %w", err)
	}

	var auth authConfig

	for authHost, authConfig := range config.Auths {
		authHost, err := normalizeHost(authHost)
		if err != nil {
			return "", "", fmt.Errorf("normalize auth host: %w", err)
		}

		if authHost == host {
			auth = authConfig
			break
		}
	}

	if auth.Auth != "" {
		username, password, err := decodeAuth(auth.Auth)
		if err != nil {
			return "", "", fmt.Errorf("decode auth: %w", err)
		}
		return username, password, nil
	}

	return auth.Username, auth.Password, nil
}

func encodeAuth(username, password string) string {
	auth := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func decodeAuth(auth string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		// Try decoding as if there's no padding
		decoded, err = base64.RawStdEncoding.DecodeString(auth)
		if err != nil {
			return "", "", fmt.Errorf("decode base64: %w", err)
		}
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format: expected 'username:password'")
	}
	return parts[0], parts[1], nil
}

func normalizeHost(host string) (string, error) {
	targetHost := host

	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		targetHost = "//" + host
	}

	u, err := url.Parse(targetHost)
	if err != nil {
		return "", fmt.Errorf("parse host %q: %w", host, err)
	}

	if u.Host == "" || strings.HasPrefix(u.Host, ":") {
		return "", fmt.Errorf("parse host %q: empty host component", host)
	}
	return u.Host, nil
}
