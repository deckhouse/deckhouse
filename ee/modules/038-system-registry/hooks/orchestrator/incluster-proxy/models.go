/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
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
			Message: "The incluster-proxy is currently active.",
		}
	}
	return StopResult{
		Ready:   true,
		Message: "The incluster-proxy has been successfully stopped.",
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
				"Incluster-proxy version is out of sync. Current version: %s, expected version: %s.",
				inputs.Version, state.Config.Version,
			),
		}
	case !inputs.IsReady:
		result = ProcessResult{
			Ready:   false,
			Message: fmt.Sprintf("Incluster-proxy is in progress: %s", inputs.ReadyMsg),
		}
	default:
		result = ProcessResult{
			Ready:   true,
			Message: "Configuration for incluster-proxy processed successfully.",
		}
	}
	return result, nil
}

func (cfg *StateConfig) process(log go_hook.Logger, params Params) error {
	if err := cfg.Config.process(log, params); err != nil {
		return err
	}

	version, err := helpers.ComputeHash(cfg.Config)
	if err != nil {
		return fmt.Errorf("cannot compute config hash: %w", err)
	}
	cfg.Version = version
	return nil
}

func (cfg *InclusterProxyConfig) process(log go_hook.Logger, params Params) error {
	upstreamUser, err := processUserPasswordHash(
		log,
		users.User{
			UserName:       params.Upstream.UserName,
			Password:       params.Upstream.Password,
			HashedPassword: cfg.Upstream.User.HashedPassword,
		})
	if err != nil {
		return fmt.Errorf("cannot process Upstream User password hash: %w", err)
	}

	authCertPair, err := processAuthCertPair(
		log,
		CertPair{Cert: cfg.AuthCert, Key: cfg.AuthKey},
		params.CA,
	)
	if err != nil {
		return fmt.Errorf("cannot process Auth cert and key: %w", err)
	}

	distributionCertPair, err := processDistributionCertPair(
		log,
		CertPair{Cert: cfg.DistributionCert, Key: cfg.DistributionKey},
		params.CA,
	)
	if err != nil {
		return fmt.Errorf("cannot process Distribution cert and key: %w", err)
	}

	tokenKey, err := pki.EncodePrivateKey(params.Token.Key)
	if err != nil {
		return fmt.Errorf("cannot encode Token key: %w", err)
	}

	var upstreamCA string
	if params.Upstream.CA != nil {
		upstreamCA = string(pki.EncodeCertificate(params.Upstream.CA))
	}

	host, path := getRegistryAddressAndPathFromImagesRepo(params.Upstream.ImagesRepo)
	*cfg = InclusterProxyConfig{
		CACert:           string(pki.EncodeCertificate(params.CA.Cert)),
		AuthCert:         authCertPair.Cert,
		AuthKey:          authCertPair.Key,
		TokenCert:        string(pki.EncodeCertificate(params.Token.Cert)),
		TokenKey:         string(tokenKey),
		DistributionCert: distributionCertPair.Cert,
		DistributionKey:  distributionCertPair.Key,
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
