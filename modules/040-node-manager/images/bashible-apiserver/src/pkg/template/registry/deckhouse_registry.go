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

package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	corev1 "k8s.io/api/core/v1"
)

const (
	deckhouseRegistrySecretName = "deckhouse-registry"
)

type deckhouseRegistrySecret struct {
	Address      string `json:"address" yaml:"address"`
	Path         string `json:"path" yaml:"path"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca,omitempty" yaml:"ca,omitempty"`
	DockerConfig []byte `json:".dockerconfigjson" yaml:".dockerconfigjson"`
}

func (d *deckhouseRegistrySecret) decode(secret *corev1.Secret) error {
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

func (d deckhouseRegistrySecret) validate() error {
	return nil
}

func (d deckhouseRegistrySecret) auth() (string, error) {
	if len(d.DockerConfig) == 0 {
		return "", nil
	}

	var cfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}

	if err := json.Unmarshal(d.DockerConfig, &cfg); err != nil {
		return "", fmt.Errorf("failed to unmarshal .dockerconfigjson: %w", err)
	}

	authInfo, ok := cfg.Auths[d.Address]
	if !ok {
		return "", nil
	}

	if authInfo.Auth != "" {
		return authInfo.Auth, nil
	}

	if authInfo.Username != "" && authInfo.Password != "" {
		raw := fmt.Sprintf("%s:%s", authInfo.Username, authInfo.Password)
		return base64.StdEncoding.EncodeToString([]byte(raw)), nil
	}
	return "", nil
}

func (d deckhouseRegistrySecret) toRegistryData() (*RegistryData, error) {
	imagesBase := d.Address
	if path := strings.TrimSpace(strings.TrimLeft(d.Path, "/")); path != "" {
		imagesBase = d.Address + "/" + path
	}

	auth, err := d.auth()
	if err != nil {
		return nil, err
	}

	ret := &RegistryData{
		RegistryModuleEnable: false,
		Mode:                 "unmanaged",
		Version:              "unknown",
		ImagesBase:           imagesBase,
		Hosts: map[string]bashible.ContextHosts{d.Address: {
			Mirrors: []bashible.ContextMirrorHost{{
				Host:   d.Address,
				Scheme: d.Scheme,
				CA:     d.CA,
				Auth: bashible.ContextAuth{
					Auth: auth,
				},
			}},
		}},
	}
	return ret, nil
}
