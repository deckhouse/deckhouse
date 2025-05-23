/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

type dockerCfg struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

func DockerCfgFromCreds(username, password, host string) ([]byte, error) {
	targetHost, err := normalizeHost(host)
	if err != nil {
		return []byte{}, err
	}

	cfg := dockerCfg{
		Auths: map[string]authn.AuthConfig{
			targetHost: {
				Username: username,
				Password: password,
				Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
			},
		},
	}
	return json.Marshal(cfg)
}

func CredsFromDockerCfg(rawCfg []byte, host string) (string, string, error) {
	if len(rawCfg) == 0 {
		return "", "", nil
	}

	// Username and password added by unmarshal
	// https://github.com/google/go-containerregistry/blob/main/pkg/authn/authn.go#L67-L94
	var cfg dockerCfg
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal docker config: %w", err)
	}

	targetHost, err := normalizeHost(host)
	if err != nil {
		return "", "", err
	}

	for repoKey, auth := range cfg.Auths {
		repoHost, err := normalizeHost(repoKey)
		if err != nil {
			return "", "", err
		}
		if repoHost == targetHost {
			return auth.Username, auth.Password, nil
		}
	}
	return "", "", nil
}

func normalizeHost(host string) (string, error) {
	targetHost := host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		targetHost = "//" + host
	}
	u, err := url.Parse(targetHost)
	if err != nil {
		return "", fmt.Errorf("failed to parse input host %q: %w", host, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("failed to parse input host %q: host component is empty", host)
	}
	return u.Host, nil
}
