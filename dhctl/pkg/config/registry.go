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

package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type RegistryData struct {
	Address   string `json:"address"`
	Path      string `json:"path"`
	Scheme    string `json:"scheme"`
	CA        string `json:"ca"`
	DockerCfg string `json:"dockerCfg"`
}

type RegistryBashibleCtx struct {
	Mode           string                           `json:"mode" yaml:"mode"`
	Version        string                           `json:"version" yaml:"version"`
	ImagesBase     string                           `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                         `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []RegistryBashibleCtxHostsObject `json:"hosts" yaml:"hosts"`
	PrepullHosts   []RegistryBashibleCtxHostsObject `json:"prepullHosts" yaml:"prepullHosts"`
}

type RegistryBashibleCtxHostsObject struct {
	Host    string                                `json:"host" yaml:"host"`
	CA      []string                              `json:"ca" yaml:"ca"`
	Mirrors []RegistryBashibleCtxMirrorHostObject `json:"mirrors" yaml:"mirrors"`
}

type RegistryBashibleCtxMirrorHostObject struct {
	Host     string `json:"host" yaml:"host"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
	Scheme   string `json:"scheme" yaml:"scheme"`
}

func (r *RegistryData) KubeadmTemplatesContext() (map[string]interface{}, error) {
	return r.ConvertToMap()
}

func (r RegistryData) BashibleBundleTemplateContext() (map[string]interface{}, error) {
	ctx, err := r.ConvertToBashibleCtx()
	if err != nil {
		return nil, err
	}
	return ctx.ConvertToMap()
}

func (r *RegistryData) Auth() (string, error) {
	type dockerCfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}

	var (
		registryAuth string
		dc           dockerCfg
	)

	bytes, err := base64.StdEncoding.DecodeString(r.DockerCfg)
	if err != nil {
		return "", fmt.Errorf("cannot base64 decode docker cfg: %v", err)
	}

	log.DebugF("parse registry data: dockerCfg after base64 decode = %s\n", bytes)
	err = json.Unmarshal(bytes, &dc)
	if err != nil {
		return "", fmt.Errorf("cannot unmarshal docker cfg: %v", err)
	}

	if registry, ok := dc.Auths[r.Address]; ok {
		switch {
		case registry.Auth != "":
			registryAuth = registry.Auth
		case registry.Username != "" && registry.Password != "":
			auth := fmt.Sprintf("%s:%s", registry.Username, registry.Password)
			registryAuth = base64.StdEncoding.EncodeToString([]byte(auth))
		default:
			log.DebugF("auth or username with password not found in dockerCfg %s for %s. Use empty string", bytes, r.Address)
		}
	}

	return registryAuth, nil
}

func (r *RegistryData) ConvertToMap() (map[string]interface{}, error) {
	ret := map[string]interface{}{
		"address":   r.Address,
		"path":      r.Path,
		"scheme":    r.Scheme,
		"ca":        r.CA,
		"dockerCfg": r.DockerCfg,
	}
	if r.DockerCfg != "" {
		auth, err := r.Auth()
		if err != nil {
			return nil, err
		}

		ret["auth"] = auth
	}
	return ret, nil
}

func (r *RegistryData) ConvertToBashibleCtx() (*RegistryBashibleCtx, error) {
	imagesBase := strings.TrimRight(r.Address, "/")
	if path := strings.TrimLeft(r.Path, "/"); path != "" {
		imagesBase += "/" + path
	}

	auth, err := r.Auth()
	if err != nil {
		return nil, err
	}

	ca := []string{}
	if r.CA != "" {
		ca = append(ca, r.CA)
	}

	mirrorHost := RegistryBashibleCtxMirrorHostObject{
		Host:   r.Address,
		Auth:   auth,
		Scheme: r.Scheme,
	}

	ret := &RegistryBashibleCtx{
		Mode:           "unmanaged",
		ImagesBase:     imagesBase,
		Version:        "unknown",
		ProxyEndpoints: []string{},
		Hosts: []RegistryBashibleCtxHostsObject{{
			Host:    r.Address,
			CA:      ca,
			Mirrors: []RegistryBashibleCtxMirrorHostObject{mirrorHost},
		}},
		PrepullHosts: []RegistryBashibleCtxHostsObject{{
			Host:    r.Address,
			CA:      ca,
			Mirrors: []RegistryBashibleCtxMirrorHostObject{mirrorHost},
		}},
	}
	return ret, nil
}

func (r *RegistryBashibleCtx) ConvertToMap() (map[string]interface{}, error) {
	jsonData, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	return result, err
}
