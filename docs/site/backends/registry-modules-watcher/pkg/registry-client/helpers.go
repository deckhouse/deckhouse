// Copyright 2023 Flant JSC
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

package registryclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

func readAuthConfig(repo, dockerCfg string) (authn.AuthConfig, error) {
	r, err := parse(repo)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("parse: %w", err)
	}

	var auths dockercfgAuths
	err = json.Unmarshal([]byte(dockerCfg), &auths)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("json unmarshal: %w", err)
	}

	// The config should have at least one .auths.* entry
	if repoAuth, ok := auths.Auths[r.Host]; ok {
		return repoAuth, nil
	}

	return authn.AuthConfig{}, errors.New("no authentication data found for the repository")
}

type dockercfgAuths struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

// parse parses url without scheme://
// if we pass url without scheme we've got url back with two leading slashes
func parse(rawURL string) (*url.URL, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return url.ParseRequestURI(rawURL)
	}

	return url.Parse("//" + rawURL)
}
