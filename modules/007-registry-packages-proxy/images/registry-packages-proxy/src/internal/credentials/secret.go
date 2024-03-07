// Copyright 2024 Flant JSC
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

package credentials

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

type registrySecretData struct {
	Address      string `json:"address" yaml:"address"`
	Path         string `json:"path" yaml:"path"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca,omitempty" yaml:"ca,omitempty"`
	DockerConfig []byte `json:".dockerconfigjson" yaml:".dockerconfigjson"`
}

func (d *registrySecretData) FromSecretData(m map[string][]byte) {
	if v, ok := m["address"]; ok {
		d.Address = string(v)
	}

	if v, ok := m["path"]; ok {
		d.Path = string(v)
	}

	if v, ok := m["scheme"]; ok {
		d.Scheme = string(v)
	}

	if v, ok := m["ca"]; ok {
		d.CA = string(v)
	}

	if v, ok := m[".dockerconfigjson"]; ok {
		d.DockerConfig = v
	}
}

func (d registrySecretData) toClientConfig() (*registry.ClientConfig, error) {
	var auth string

	if len(d.DockerConfig) > 0 {
		var err error
		auth, err = dockerConfigToAuth(d.DockerConfig, d.Address)
		if err != nil {
			return nil, err
		}
	}

	return &registry.ClientConfig{
		Repository: strings.Join([]string{d.Address, d.Path}, "/"),
		Scheme:     d.Scheme,
		CA:         d.CA,
		Auth:       auth,
	}, nil
}

type dockerConfig struct {
	Auths map[string]struct {
		Auth     string `json:"auth"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"auths"`
}

func dockerConfigToAuth(config []byte, address string) (string, error) {
	var dockerConfig dockerConfig

	err := json.Unmarshal(config, &dockerConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal docker config")
	}

	if registryObj, ok := dockerConfig.Auths[address]; ok {
		switch {
		case registryObj.Auth != "":
			return registryObj.Auth, nil
		case registryObj.Username != "" && registryObj.Password != "":
			authRaw := fmt.Sprintf("%s:%s", registryObj.Username, registryObj.Password)
			return base64.StdEncoding.EncodeToString([]byte(authRaw)), nil
		}
	}

	return "", nil
}
