// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

func AuthFromDockerConfig(repo, dockerCfgBase64 string) (authn.Authenticator, error) {
	authConfig, err := readAuthConfig(repo, dockerCfgBase64)
	if err != nil {
		return nil, fmt.Errorf("read auth config: %w", err)
	}

	if authConfig.Username == "" && authConfig.Password == "" {
		return authn.Anonymous, nil
	}

	return &authn.Basic{
		Username: authConfig.Username,
		Password: authConfig.Password,
	}, nil
}

// dockerConfig represents the structure of a Docker config JSON
type dockerConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

func readAuthConfig(repo, dockerCfgBase64 string) (authn.AuthConfig, error) {
	r, err := parse(repo)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("parse repo: %w", err)
	}

	dockerCfg, err := base64.StdEncoding.DecodeString(dockerCfgBase64)
	if err != nil {
		// if base64 decoding failed, try to use input as it is
		dockerCfg = []byte(dockerCfgBase64)
	}

	var config dockerConfig
	if err := json.Unmarshal(dockerCfg, &config); err != nil {
		return authn.AuthConfig{}, fmt.Errorf("unmarshal docker config: %w", err)
	}

	// The config should have at least one .auths.* entry
	for repoName, repoAuth := range config.Auths {
		repoNameURL, err := parse(repoName)
		if err != nil {
			return authn.AuthConfig{}, fmt.Errorf("parse repo name: %w", err)
		}

		if repoNameURL.Host == r.Host {
			return repoAuth, nil
		}
	}

	return authn.AuthConfig{}, fmt.Errorf("%q credentials not found in the dockerCfg", repo)
}

// parse parses url without scheme://
// if we pass url without scheme ve've got url back with two leading slashes
func parse(rawURL string) (*url.URL, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return url.ParseRequestURI(rawURL)
	}
	return url.Parse("//" + rawURL)
}
