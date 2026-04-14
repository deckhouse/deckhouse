/*
Copyright 2026 Flant JSC

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

package pki

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

// config holds all parameters required to generate a Kubernetes PKI.
// Required fields are passed explicitly to newConfig; optional settings
// are applied via WithXxx functional options.
type config struct {
	// NodeName is the node hostname.
	// Added as a DNS SAN to apiserver (if not an IP address), etcd/server, and etcd/peer.
	// Used as CommonName in etcd/server and etcd/peer certificates.
	// kubeadm equivalent: NodeRegistrationOptions.Name
	NodeName string

	// DNSDomain is the cluster DNS domain, e.g. "cluster.local".
	// Used to build the SAN "kubernetes.default.svc.<DNSDomain>" in the apiserver certificate.
	// kubeadm equivalent: ClusterConfiguration.Networking.DNSDomain
	DNSDomain string

	// AdvertiseAddress is the node IP on which the API server listens.
	// Added as an IP SAN to apiserver, etcd/server, and etcd/peer.
	// kubeadm equivalent: InitConfiguration.LocalAPIEndpoint.AdvertiseAddress
	AdvertiseAddress net.IP

	// ServiceCIDR is the CIDR range for cluster service IPs.
	// The first IP in this range (the ClusterIP of the "kubernetes" service) is added
	// as an IP SAN to the apiserver certificate.
	// Example: "10.96.0.0/12" → 10.96.0.1 is added.
	// kubeadm equivalent: ClusterConfiguration.Networking.ServiceSubnet
	ServiceCIDR string

	// CertTreeScheme defines the PKI tree structure: which CAs exist and
	// which leaf certificates are signed by each of them.
	// nil falls back to defaultCertTreeScheme (full set including etcd).
	CertTreeScheme map[RootCertName][]LeafCertName

	// pkiDir is the directory where certificates and keys are written.
	// Default: constants.DefaultCertificatesDir (/etc/kubernetes/pki).
	pkiDir string

	// ControlPlaneEndpoint is the external HA endpoint of the cluster ("hostname:port" or "IP:port").
	// The host part is extracted and added as a SAN to the apiserver certificate.
	// kubeadm equivalent: ClusterConfiguration.ControlPlaneEndpoint
	ControlPlaneEndpoint string

	// APIServerCertSANs is a list of additional Subject Alternative Names for the apiserver certificate.
	// Each entry is automatically classified as an IP address or a DNS name.
	// kubeadm equivalent: ClusterConfiguration.APIServer.CertSANs
	APIServerCertSANs []string

	// EtcdServerCertSANs is a list of additional SANs for the etcd/server certificate.
	// kubeadm equivalent: InitConfiguration.Etcd.Local.ServerCertSANs
	EtcdServerCertSANs []string

	// EtcdPeerCertSANs is a list of additional SANs for the etcd/peer certificate.
	// kubeadm equivalent: InitConfiguration.Etcd.Local.PeerCertSANs
	EtcdPeerCertSANs []string

	// EncryptionAlgorithmType is the asymmetric encryption algorithm used for all generated keys.
	// Applied uniformly to all certificates and the SA key pair.
	// Default: constants.EncryptionAlgorithmRSA2048.
	// kubeadm equivalent: ClusterConfiguration.EncryptionAlgorithm
	EncryptionAlgorithmType constants.EncryptionAlgorithmType

	// CertValidityPeriod is the validity duration for leaf (non-CA) certificates.
	// Default: constants.CertificateValidityPeriod (1 year).
	CertValidityPeriod time.Duration

	// CACertValidityPeriod is the validity duration for CA certificates.
	// Default: constants.CACertificateValidityPeriod (10 years).
	CACertValidityPeriod time.Duration
}

// newConfig constructs and validates a config.
// The required fields are validated eagerly and all errors are joined before returning.
// Optional settings are applied via configOptions; defaults are filled in afterwards.
func newConfig(
	nodeName string,
	dnsDomain string,
	advertiseAddress net.IP,
	serviceCIDR string,
	configOptions ...configOption) (*config, error) {
	var errs []error

	if nodeName == "" {
		errs = append(errs, fmt.Errorf("NodeName is required but not provided"))
	}

	if dnsDomain == "" {
		errs = append(errs, fmt.Errorf("DNSDomain is required but not provided"))
	}

	if len(advertiseAddress) == 0 {
		errs = append(errs, fmt.Errorf("AdvertiseAddress is required but not provided"))
	}

	if serviceCIDR == "" {
		errs = append(errs, fmt.Errorf("ServiceCIDR is required but not provided"))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	cfg := &config{
		NodeName:         nodeName,
		DNSDomain:        dnsDomain,
		AdvertiseAddress: advertiseAddress,
		ServiceCIDR:      serviceCIDR,
	}

	for _, configOption := range configOptions {
		configOption(cfg)
	}

	if cfg.CertTreeScheme == nil {
		cfg.CertTreeScheme = defaultCertTreeScheme
	}

	if cfg.pkiDir == "" {
		cfg.pkiDir = constants.DefaultCertificatesDir
	}

	if cfg.EncryptionAlgorithmType == "" {
		cfg.EncryptionAlgorithmType = constants.EncryptionAlgorithmRSA2048
	}

	if cfg.CertValidityPeriod == 0 {
		cfg.CertValidityPeriod = constants.CertificateValidityPeriod
	}

	if cfg.CACertValidityPeriod == 0 {
		cfg.CACertValidityPeriod = constants.CACertificateValidityPeriod
	}

	return cfg, nil
}

// configOption is a functional option for PKI configuration.
// It supplies optional parameters to CreatePKIBundle after the required arguments.
type configOption func(*config)

// WithCertTreeScheme overrides the PKI tree structure.
// Use this to generate only a subset of certificates, for example when running
// in etcd arbiter mode where only etcd certificates need to be renewed:
//
//	scheme := map[pki.RootCertName][]pki.LeafCertName{
//	    pki.EtcdCACertName: {pki.EtcdServerCertName, pki.EtcdPeerCertName, pki.EtcdHealthcheckClientCertName},
//	}
//	CreatePKIBundle(..., WithCertTreeScheme(scheme))
func WithCertTreeScheme(certTreeScheme map[RootCertName][]LeafCertName) configOption {
	return func(c *config) {
		c.CertTreeScheme = certTreeScheme
	}
}

// WithPKIDir overrides the directory where certificates and keys are written.
// Useful in tests or when using a non-standard PKI location.
func WithPKIDir(pkiDir string) configOption {
	return func(c *config) {
		c.pkiDir = pkiDir
	}
}

// WithControlPlaneEndpoint sets the external HA endpoint of the cluster.
// Format: "hostname:port" or "IP:port". The port is stripped; the host is added as a SAN to apiserver.
func WithControlPlaneEndpoint(endpoint string) configOption {
	return func(c *config) {
		c.ControlPlaneEndpoint = endpoint
	}
}

// WithAPIServerCertSANs sets additional SANs for the kube-apiserver certificate.
func WithAPIServerCertSANs(sans []string) configOption {
	return func(c *config) {
		c.APIServerCertSANs = sans
	}
}

// WithEtcdServerCertSANs sets additional SANs for the etcd/server certificate.
func WithEtcdServerCertSANs(sans []string) configOption {
	return func(c *config) {
		c.EtcdServerCertSANs = sans
	}
}

// WithEtcdPeerCertSANs sets additional SANs for the etcd/peer certificate.
func WithEtcdPeerCertSANs(sans []string) configOption {
	return func(c *config) {
		c.EtcdPeerCertSANs = sans
	}
}

// WithEncryptionAlgorithmType sets the encryption algorithm used for all generated keys.
func WithEncryptionAlgorithmType(algo constants.EncryptionAlgorithmType) configOption {
	return func(c *config) {
		c.EncryptionAlgorithmType = algo
	}
}

// WithCertValidityPeriod sets the validity duration for leaf certificates.
func WithCertValidityPeriod(duration time.Duration) configOption {
	return func(c *config) {
		c.CertValidityPeriod = duration
	}
}

// WithCACertValidityPeriod sets the validity duration for CA certificates.
func WithCACertValidityPeriod(duration time.Duration) configOption {
	return func(c *config) {
		c.CACertValidityPeriod = duration
	}
}

// certConfig is the per-certificate configuration type used throughout pki2.
// It is a type alias for pkiutil.CertConfig so that pki2 can construct values
// with the familiar struct-literal syntax while the actual type lives in pkiutil.
type certConfig = pkiutil.CertConfig
