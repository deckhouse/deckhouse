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
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type RegistryData struct {
	Address   string `json:"address"`
	Path      string `json:"path"`
	Scheme    string `json:"scheme"`
	CA        string `json:"ca"`
	DockerCfg string `json:"dockerCfg"`
}

func (r *RegistryData) Process(cfg DeckhouseClusterConfig) error {
	parts := strings.SplitN(cfg.ImagesRepo, "/", 2)
	r.Address = parts[0]
	if len(parts) == 2 {
		r.Path = fmt.Sprintf("/%s", parts[1])
	}

	if err := validateRegistryDockerCfg(cfg.RegistryDockerCfg, r.Address); err != nil {
		return err
	}
	r.DockerCfg = cfg.RegistryDockerCfg
	r.Scheme = strings.ToLower(cfg.RegistryScheme)
	r.CA = cfg.RegistryCA
	if err := validateHTTPRegistryScheme(r.Scheme, r.CA); err != nil {
		return err
	}
	return nil
}

func (r *RegistryData) KubeadmTemplatesCtx() (map[string]interface{}, error) {
	return r.toMap()
}

func (r *RegistryData) BashibleBundleTemplateCtx() (map[string]interface{}, error) {
	ret, err := r.toBashibleCtx()
	if err != nil {
		return nil, err
	}
	if ret.Validate() != nil {
		return nil, err
	}
	return ret.ToMap()
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

func (r *RegistryData) toMap() (map[string]interface{}, error) {
	log.DebugF("registry data: %v\n", r)

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

func (r *RegistryData) toBashibleCtx() (*bashible.Context, error) {
	log.DebugF("registry data: %v\n", r)

	imagesBase := r.Address
	if path := strings.TrimSpace(strings.TrimLeft(r.Path, "/")); path != "" {
		imagesBase = r.Address + "/" + path
	}

	var (
		auth string
		err  error
	)
	if r.DockerCfg != "" {
		auth, err = r.Auth()
		if err != nil {
			return nil, err
		}
	}

	ret := &bashible.Context{
		RegistryModuleEnable: false,
		Mode:                 "unmanaged",
		Version:              "unknown",
		ImagesBase:           imagesBase,
		ProxyEndpoints:       []string{},
		Hosts: map[string]bashible.ContextHosts{
			r.Address: {
				Mirrors: []bashible.ContextMirrorHost{{
					Host:   r.Address,
					CA:     r.CA,
					Scheme: r.Scheme,
					Auth: bashible.ContextAuth{
						Auth: auth,
					},
				}},
			},
		},
	}
	return ret, nil
}

func validateRegistryDockerCfg(cfg string, repo string) error {
	if cfg == "" {
		return fmt.Errorf("can't be empty")
	}

	regcrd, err := base64.StdEncoding.DecodeString(cfg)
	if err != nil {
		return fmt.Errorf("unable to decode registryDockerCfg: %w", err)
	}

	var creds struct {
		Auths map[string]interface{} `json:"auths"`
	}

	if err = json.Unmarshal(regcrd, &creds); err != nil {
		return fmt.Errorf("unable to unmarshal docker credentials: %w", err)
	}

	// The regexp match string with this pattern:
	// ^([a-z]|\d)+ - string starts with a [a-z] letter or a number
	// (\.?|\-?) - next symbol might be '.' or '-' and repeated zero or one times
	// (([a-z]|\d)+(\.|\-|))* - middle part of string might have [a-z] letters, numbers, '.' or ':',
	// and moreover '.' or ':' symbols can't be doubled or goes next to each other
	// ([a-z]|\d+|([a-z]|\d)\:\d+)$ - string might be ended by [a-z] letter or number (if we have single host) or
	// [a-z] letter or number with ':' symbol, and moreover there might be only numbers after ':' symbol
	regx, err := regexp.Compile(`^([a-z]|\d)+(\.?|\-?)(([a-z]|\d)+(\.|\-|))*([a-z]|\d+|([a-z]|\d)\:\d+)$`)
	if err != nil {
		return fmt.Errorf("unable to compile regexp by pattern: %w", err)
	}

	for k := range creds.Auths {
		if !regx.MatchString(k) {
			return fmt.Errorf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", k)
		}
	}

	for k := range creds.Auths {
		if k == repo {
			return nil
		}
	}
	return fmt.Errorf("incorrect registryDockerCfg. It must contain auths host {\"auths\": { \"%s\": {}}}", repo)
}

func validateHTTPRegistryScheme(scheme string, CA string) error {
	if strings.ToLower(scheme) == "http" && len(CA) > 0 {
		return fmt.Errorf("registry CA is not allowed for HTTP scheme")
	}
	return nil
}
