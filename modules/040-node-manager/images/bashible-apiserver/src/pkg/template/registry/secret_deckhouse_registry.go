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

package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	DeckhouseRegistrySecretName = "deckhouse-registry"
)

type deckhouseRegistry struct {
	Address      string `json:"address" yaml:"address"`
	Path         string `json:"path" yaml:"path"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca,omitempty" yaml:"ca,omitempty"`
	DockerConfig []byte `json:".dockerconfigjson" yaml:".dockerconfigjson"`
}

func (d *deckhouseRegistry) DecodeSecret(secret *corev1.Secret) error {
	if v, ok := secret.Data["address"]; ok {
		d.Address = string(v)
	}
	if v, ok := secret.Data["path"]; ok {
		d.Path = string(v)
	}

	if v, ok := secret.Data["scheme"]; ok {
		d.Scheme = string(v)
	}

	if v, ok := secret.Data["ca"]; ok {
		d.CA = string(v)
	}

	if v, ok := secret.Data[".dockerconfigjson"]; ok {
		d.DockerConfig = v
	}
	return nil
}

func (d *deckhouseRegistry) Validate() error {
	return nil
}

func (d *deckhouseRegistry) Auth() (string, error) {
	type dockerCfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}

	var auth string
	if len(d.DockerConfig) > 0 {
		var dcfg dockerCfg
		err := json.Unmarshal(d.DockerConfig, &dcfg)
		if err != nil {
			return "", fmt.Errorf("error: %w", err)
		}

		if registryObj, ok := dcfg.Auths[d.Address]; ok {
			switch {
			case registryObj.Auth != "":
				auth = registryObj.Auth
			case registryObj.Username != "" && registryObj.Password != "":
				authRaw := fmt.Sprintf("%s:%s", registryObj.Username, registryObj.Password)
				auth = base64.StdEncoding.EncodeToString([]byte(authRaw))
			}
		}
	}
	return auth, nil
}

func (d deckhouseRegistry) ConvertToRegistryData() (*RegistryData, error) {
	imagesBase := strings.TrimRight(d.Address, "/")
	if path := strings.TrimLeft(d.Path, "/"); path != "" {
		imagesBase += "/" + path
	}

	auth, err := d.Auth()
	if err != nil {
		return nil, err
	}

	ca := []string{}
	if d.CA != "" {
		ca = append(ca, d.CA)
	}

	mirrorHost := RegistryDataMirrorHostObject{
		Host:   d.Address,
		Auth:   auth,
		Scheme: d.Scheme,
	}

	ret := &RegistryData{
		Mode:           "unmanaged",
		ImagesBase:     imagesBase,
		Version:        "unknown",
		ProxyEndpoints: []string{},
		Hosts: []RegistryDataHostsObject{{
			Host:    d.Address,
			CA:      ca,
			Mirrors: []RegistryDataMirrorHostObject{mirrorHost},
		}},
		PrepullHosts: []RegistryDataHostsObject{{
			Host:    d.Address,
			CA:      ca,
			Mirrors: []RegistryDataMirrorHostObject{mirrorHost},
		}},
	}
	return ret, nil
}
