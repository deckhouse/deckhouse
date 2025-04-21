/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"crypto/x509"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/node-services"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type Inputs struct {
	Nodes map[string]Node
}

type Params struct {
	CA         pki.CertKey
	Token      pki.CertKey
	HTTPSecret string
	UserRO     users.User

	Proxy *ProxyModeParams
	Local *LocalModeParams
}

type ProxyModeParams struct {
	Scheme     string
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string

	UpstreamCA *x509.Certificate
}

type LocalModeParams struct {
	UserRW     users.User
	UserPuller users.User
	UserPusher users.User

	IngressCA *x509.Certificate
}

type State struct {
	Nodes map[string]NodeServicesConfig
}

func (state *State) Process(log go_hook.Logger, params Params, inputs Inputs) (bool, error) {
	var (
		nodesIP []string
		err     error
		ready   = true
	)

	if params.Local != nil {
		nodesIPSet := make(map[string]struct{})
		for _, node := range inputs.Nodes {
			nodesIPSet[node.IP] = struct{}{}
		}

		nodesIP := make([]string, 0, len(nodesIPSet))
		for ip := range nodesIPSet {
			nodesIP = append(nodesIP, ip)
		}
		sort.Strings(nodesIP)
	}

	if state.Nodes == nil {
		state.Nodes = make(map[string]NodeServicesConfig)
	}

	for name, node := range inputs.Nodes {
		config := state.Nodes[name]

		err = config.process(log, name, node, params, nodesIP)
		if err != nil {
			return false, fmt.Errorf("cannot process node %v config: %w", name, err)
		}

		state.Nodes[name] = config

		podReady := false
		for _, pod := range node.Pods {
			if pod.Ready && pod.Version == config.Version {
				pod.Ready = true
				break
			}
		}

		ready = ready && podReady
	}

	return ready, nil
}

type NodeServicesConfig struct {
	Version string              `json:"version"`
	Config  nodeservices.Config `json:"config"`
}

func (nsc *NodeServicesConfig) process(log go_hook.Logger, name string, node Node, params Params, nodesIP []string) error {
	switch {
	case params.Local != nil:
		nsc.processLocalMode(*params.Local, node.IP, nodesIP)
	case params.Proxy != nil:
		nsc.processProxyMode(*params.Proxy)
	default:
		return errors.New("params must be set for Local or Proxy mode")
	}

	err := nsc.processPKI(log, name, node.IP, params)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	nsc.Config.HTTPSecret = params.HTTPSecret
	nsc.Config.UserRO = mapUser(params.UserRO)

	nsc.Version, err = helpers.ComputeHash(nsc.Config)
	if err != nil {
		return fmt.Errorf("cannot compute config hash: %w", err)
	}

	return fmt.Errorf("not implemented")
}

func (nsc *NodeServicesConfig) processLocalMode(params LocalModeParams, nodeIP string, nodesIP []string) {
	cfg := nodeservices.LocalMode{
		UserRW:     mapUser(params.UserRW),
		UserPuller: mapUser(params.UserPuller),
		UserPusher: mapUser(params.UserPusher),
	}

	cfg.Upstreams = make([]string, 0, len(nodesIP))
	for _, ip := range nodesIP {
		if ip != nodeIP {
			cfg.Upstreams = append(cfg.Upstreams, ip)
		}
	}

	if params.IngressCA != nil {
		cfg.IngressClientCACert = string(pki.EncodeCertificate(params.IngressCA))
	}

	nsc.Config.LocalMode = &cfg
}

func (nsc *NodeServicesConfig) processProxyMode(params ProxyModeParams) {
	host, path := getRegistryAddressAndPathFromImagesRepo(params.ImagesRepo)

	cfg := nodeservices.ProxyMode{
		Upstream: nodeservices.UpstreamRegistry{
			Scheme:   strings.ToLower(params.Scheme),
			Host:     host,
			Path:     path,
			User:     params.UserName,
			Password: params.Password,
		},
	}

	if params.TTL != "" {
		cfg.Upstream.TTL = &params.TTL
	}

	if params.UpstreamCA != nil {
		cfg.UpstreamRegistryCACert = string(pki.EncodeCertificate(params.UpstreamCA))
	}

	nsc.Config.ProxyMode = &cfg
}

func (nsc *NodeServicesConfig) processPKI(log go_hook.Logger, name, nodeIP string, params Params) error {
	var (
		err     error
		nodePKI nodePKI
	)

	err = nodePKI.Process(log, params.CA, name, nodeIP, nsc.Config)
	if err != nil {
		return fmt.Errorf("cannot process node PKI: %w", err)
	}

	tokenKey, err := pki.EncodePrivateKey(params.Token.Key)
	if err != nil {
		return fmt.Errorf("cannot encode Token key: %w", err)
	}

	authKey, err := pki.EncodePrivateKey(nodePKI.Auth.Key)
	if err != nil {
		return fmt.Errorf("cannot encode node's Auth key: %w", err)
	}

	distributionKey, err := pki.EncodePrivateKey(nodePKI.Distribution.Key)
	if err != nil {
		return fmt.Errorf("cannot encode node's Distribution key: %w", err)
	}

	cfg := nsc.Config

	cfg.CACert = string(pki.EncodeCertificate(params.CA.Cert))

	cfg.TokenCert = string(pki.EncodeCertificate(params.Token.Cert))
	cfg.TokenKey = string(tokenKey)

	cfg.AuthCert = string(pki.EncodeCertificate(nodePKI.Auth.Cert))
	cfg.AuthKey = string(authKey)

	cfg.DistributionCert = string(pki.EncodeCertificate(nodePKI.Distribution.Cert))
	cfg.DistributionKey = string(distributionKey)

	nsc.Config = cfg
	return nil
}

type Pod struct {
	Ready   bool
	Version string
}

type hookPod struct {
	Pod
	Node string
}

type NodePods map[string]Pod

type Node struct {
	IP    string   `json:"ip,omitempty"`
	Ready bool     `json:"ready,omitempty"`
	Pods  NodePods `json:"pods,omitempty"`
}
