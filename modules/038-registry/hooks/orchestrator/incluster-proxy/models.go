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

package inclusterproxy

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

type Inputs = DeploymentStatus

type DeploymentStatus struct {
	IsExist  bool
	IsReady  bool
	ReadyMsg string
	Version  string
}

type Params struct {
	CA         pki.CertKey
	Token      pki.CertKey
	HTTPSecret string
	Upstream   UpstreamParams
}

type UpstreamParams struct {
	Scheme     string
	ImagesRepo string
	UserName   string
	Password   string
	CA         *x509.Certificate
}

type ProcessResult struct {
	Ready   bool
	Message string
}

type StopResult struct {
	Ready   bool
	Message string
}

type State struct {
	Config *StateConfig `json:"config,omitempty"`
}

type StateConfig struct {
	Version string               `json:"version,omitempty"`
	Config  InclusterProxyConfig `json:"config,omitempty"`
}

type InclusterProxyConfig struct {
	CACert           string                 `json:"ca" yaml:"ca"`
	AuthCert         string                 `json:"auth_cert" yaml:"auth_cert"`
	AuthKey          string                 `json:"auth_key" yaml:"auth_key"`
	TokenCert        string                 `json:"token_cert" yaml:"token_cert"`
	TokenKey         string                 `json:"token_key" yaml:"token_key"`
	DistributionCert string                 `json:"distribution_cert" yaml:"distribution_cert"`
	DistributionKey  string                 `json:"distribution_key" yaml:"distribution_key"`
	HTTPSecret       string                 `json:"http_secret" yaml:"http_secret"`
	Upstream         UpstreamRegistryConfig `json:"upstream" yaml:"upstream"`
}

type UpstreamRegistryConfig struct {
	Scheme string     `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Host   string     `json:"host,omitempty" yaml:"host,omitempty"`
	Path   string     `json:"path,omitempty" yaml:"path,omitempty"`
	User   users.User `json:"user,omitempty" yaml:"user,omitempty"`
	CACert string     `json:"ca,omitempty" yaml:"ca,omitempty"`
}

func (state *State) Stop(inputs Inputs) StopResult {
	state.Config = nil
	if inputs.IsExist {
		return StopResult{
			Ready:   false,
			Message: "Stopping incluster-proxy...",
		}
	}
	return StopResult{
		Ready:   true,
		Message: "Incluster-proxy stopped successfully.",
	}
}

func (state *State) Process(log go_hook.Logger, params Params, inputs Inputs) (ProcessResult, error) {
	if state.Config == nil {
		state.Config = &StateConfig{}
	}

	if err := state.Config.process(log, params); err != nil {
		result := ProcessResult{
			Ready:   false,
			Message: "Configuration processing for incluster-proxy failed.",
		}
		return result, fmt.Errorf("cannot process config: %w", err)
	}

	var result ProcessResult

	switch {
	case !inputs.IsExist:
		result = ProcessResult{
			Ready:   false,
			Message: "Deploying incluster-proxy...",
		}
	case inputs.Version != state.Config.Version:
		result = ProcessResult{
			Ready: false,
			Message: fmt.Sprintf(
				"Incluster-proxy deployment version mismatch: current %s, expected %s.",
				inputs.Version, state.Config.Version,
			),
		}
	case !inputs.IsReady:
		result = ProcessResult{
			Ready:   false,
			Message: fmt.Sprintf("Incluster-proxy deploying in progress: %s.", inputs.ReadyMsg),
		}
	default:
		result = ProcessResult{
			Ready:   true,
			Message: "Incluster-proxy deployed successfully.",
		}
	}
	return result, nil
}

func (cfg *StateConfig) process(log go_hook.Logger, params Params) error {
	if err := cfg.Config.process(log, params); err != nil {
		return err
	}

	version, err := pki.ComputeHash(cfg.Config)
	if err != nil {
		return fmt.Errorf("cannot compute config hash: %w", err)
	}
	cfg.Version = version
	return nil
}

func (cfg *InclusterProxyConfig) process(log go_hook.Logger, params Params) error {
	upstreamUser := users.User{
		UserName:       params.Upstream.UserName,
		Password:       params.Upstream.Password,
		HashedPassword: cfg.Upstream.User.HashedPassword,
	}
	if err := processUserPasswordHash(log, &upstreamUser); err != nil {
		return fmt.Errorf("cannot process Upstream User password hash: %w", err)
	}

	inclusterProxyPKI := inclusterProxyPKI{}
	if err := inclusterProxyPKI.Process(log, params.CA, *cfg); err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	tokenKey, err := pki.EncodePrivateKey(params.Token.Key)
	if err != nil {
		return fmt.Errorf("cannot encode Token key: %w", err)
	}

	authKey, err := pki.EncodePrivateKey(inclusterProxyPKI.Auth.Key)
	if err != nil {
		return fmt.Errorf("cannot encode Auth key: %w", err)
	}

	distributionKey, err := pki.EncodePrivateKey(inclusterProxyPKI.Distribution.Key)
	if err != nil {
		return fmt.Errorf("cannot encode Distribution key: %w", err)
	}

	var upstreamCA string
	if params.Upstream.CA != nil {
		upstreamCA = string(pki.EncodeCertificate(params.Upstream.CA))
	}

	host, path := helpers.RegistryAddressAndPathFromImagesRepo(params.Upstream.ImagesRepo)
	*cfg = InclusterProxyConfig{
		CACert:           string(pki.EncodeCertificate(params.CA.Cert)),
		TokenCert:        string(pki.EncodeCertificate(params.Token.Cert)),
		TokenKey:         string(tokenKey),
		AuthCert:         string(pki.EncodeCertificate(inclusterProxyPKI.Auth.Cert)),
		AuthKey:          string(authKey),
		DistributionCert: string(pki.EncodeCertificate(inclusterProxyPKI.Distribution.Cert)),
		DistributionKey:  string(distributionKey),
		HTTPSecret:       params.HTTPSecret,
		Upstream: UpstreamRegistryConfig{
			Scheme: strings.ToLower(params.Upstream.Scheme),
			Host:   host,
			Path:   path,
			User:   upstreamUser,
			CACert: upstreamCA,
		},
	}
	return nil
}
