/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"crypto/x509"
	"fmt"
	"sort"

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
	UpstreamCA *x509.Certificate
}

type LocalModeParams struct {
	IngressCA *x509.Certificate
	UserRW    users.User
}

type State struct {
	Nodes map[string]NodeServicesConfig
}

func (state *State) Process(log go_hook.Logger, params Params, inputs Inputs) (bool, error) {
	var (
		err   error
		ready = true
	)

	if state.Nodes == nil {
		state.Nodes = make(map[string]NodeServicesConfig)
	}

	nodesIP := make([]string, 0, len(inputs.Nodes))
	for _, node := range inputs.Nodes {
		nodesIP = append(nodesIP, node.IP)
	}
	sort.Strings(nodesIP)

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

func (config *NodeServicesConfig) process(log go_hook.Logger, name string, node Node, params Params, nodesIP []string) error {
	err := config.processPKI(log, name, node.IP, params)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	config.Version, err = helpers.ComputeHash(config.Config)
	if err != nil {
		return fmt.Errorf("cannot compute config hash: %w", err)
	}

	return fmt.Errorf("not implemented")
}

func (config *NodeServicesConfig) processPKI(log go_hook.Logger, name, nodeIP string, params Params) error {
	var (
		err     error
		nodePKI nodePKI
	)

	err = nodePKI.Process(log, params.CA, name, nodeIP, config.Config.PKI)
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

	value := nodeservices.PKI{
		CACert:           string(pki.EncodeCertificate(params.CA.Cert)),
		TokenCert:        string(pki.EncodeCertificate(params.Token.Cert)),
		TokenKey:         string(tokenKey),
		AuthCert:         string(pki.EncodeCertificate(nodePKI.Auth.Cert)),
		AuthKey:          string(authKey),
		DistributionCert: string(pki.EncodeCertificate(nodePKI.Distribution.Cert)),
		DistributionKey:  string(distributionKey),
	}

	if params.Local != nil && params.Local.IngressCA != nil {
		value.IngressClientCACert = string(pki.EncodeCertificate(params.Local.IngressCA))
	}

	if params.Proxy != nil && params.Proxy.UpstreamCA != nil {
		value.UpstreamRegistryCACert = string(pki.EncodeCertificate(params.Proxy.UpstreamCA))
	}

	config.Config.PKI = value

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
