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

package controlplaneoperation

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
)

// certTreeForComponent returns the cert tree scheme for a given component
func certTreeForComponent(c controlplanev1alpha1.OperationComponent) map[pki.RootCertName][]pki.LeafCertName {
	switch c {
	case controlplanev1alpha1.OperationComponentEtcd:
		return map[pki.RootCertName][]pki.LeafCertName{
			pki.EtcdCACertName: {
				pki.EtcdServerCertName,
				pki.EtcdPeerCertName,
				pki.EtcdHealthcheckClientCertName,
				pki.ApiserverEtcdClientCertName,
			},
		}
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return map[pki.RootCertName][]pki.LeafCertName{
			pki.CACertName: {
				pki.ApiserverCertName,
				pki.ApiserverKubeletClientCertName,
			},
			pki.FrontProxyCACertName: {
				pki.FrontProxyClientCertName,
			},
		}
	default:
		return nil
	}
}

// componentLeafCertFiles maps components to leaf cert base names (relative to pki dir, no extension).
var componentLeafCertFiles = map[controlplanev1alpha1.OperationComponent][]string{
	controlplanev1alpha1.OperationComponentEtcd: {
		"etcd/server",
		"etcd/peer",
		"etcd/healthcheck-client",
		"apiserver-etcd-client",
	},
	controlplanev1alpha1.OperationComponentKubeAPIServer: {
		"apiserver",
		"apiserver-kubelet-client",
		"front-proxy-client",
	},
}

// componentCAFiles maps components to the CA/SA file paths (relative to pki dir), derived from its cert tree.
var componentCAFiles = map[controlplanev1alpha1.OperationComponent][]string{
	controlplanev1alpha1.OperationComponentEtcd: {
		"etcd/ca.crt",
		"etcd/ca.key",
	},
	controlplanev1alpha1.OperationComponentKubeAPIServer: {
		"ca.crt",
		"ca.key",
		"front-proxy-ca.crt",
		"front-proxy-ca.key",
		"sa.pub",
		"sa.key",
	},
}

var caFileMapping = map[string]string{
	"ca.crt":             "ca.crt",
	"ca.key":             "ca.key",
	"front-proxy-ca.crt": "front-proxy-ca.crt",
	"front-proxy-ca.key": "front-proxy-ca.key",
	"sa.pub":             "sa.pub",
	"sa.key":             "sa.key",
	"etcd-ca.crt":        "etcd/ca.crt",
	"etcd-ca.key":        "etcd/ca.key",
}

// installCAsFromSecret copies CA certs and SA keys from d8-pki secret to disk.
func installCAsFromSecret(pkiSecretData map[string][]byte, pkiDir string) error {
	etcdDir := filepath.Join(pkiDir, "etcd")
	if err := os.MkdirAll(etcdDir, 0o700); err != nil {
		return fmt.Errorf("create etcd pki dir: %w", err)
	}

	for secretKey, relPath := range caFileMapping {
		content, ok := pkiSecretData[secretKey]
		if !ok {
			continue
		}
		dst := filepath.Join(pkiDir, relPath)
		if err := writeFileAtomically(dst, content, 0o600); err != nil {
			return fmt.Errorf("write CA file %s: %w", relPath, err)
		}
	}

	return nil
}

// PKIParams holds parameters for certificate renewal.
type PKIParams struct {
	NodeName          string
	AdvertiseAddress  string
	ClusterDomain     string
	ServiceSubnetCIDR string
	PKIDir            string

	ApiServerCertSANs   []string
	EncryptionAlgorithm string
}

// renewCertsIfNeeded calls pki.CreatePKIBundle which is idempotent using the following rules:
// - existing valid leaf certs are kept
// - leaf certs regenerated only if: missing, expires < 30 days, or SANs changed
// - CA certs are never auto-regenerated
// certTree limits the cert tree to a specific component's certs (if nil - the full tree is used)
func renewCertsIfNeeded(params PKIParams, certTree map[pki.RootCertName][]pki.LeafCertName) error {
	ip := net.ParseIP(params.AdvertiseAddress)
	if ip == nil {
		return fmt.Errorf("invalid advertise address: %s", params.AdvertiseAddress)
	}

	if params.EncryptionAlgorithm == "" {
		if certTree != nil {
			return pki.CreatePKIBundle(
				params.NodeName, params.ClusterDomain, ip, params.ServiceSubnetCIDR,
				pki.WithPKIDir(params.PKIDir),
				pki.WithControlPlaneEndpoint(constants.LocalControlPlaneEndpoint),
				pki.WithAPIServerCertSANs(params.ApiServerCertSANs),
				pki.WithCertTreeScheme(certTree),
			)
		}
		return pki.CreatePKIBundle(
			params.NodeName, params.ClusterDomain, ip, params.ServiceSubnetCIDR,
			pki.WithPKIDir(params.PKIDir),
			pki.WithControlPlaneEndpoint(constants.LocalControlPlaneEndpoint),
			pki.WithAPIServerCertSANs(params.ApiServerCertSANs),
		)
	}

	if certTree != nil {
		return pki.CreatePKIBundle(
			params.NodeName, params.ClusterDomain, ip, params.ServiceSubnetCIDR,
			pki.WithPKIDir(params.PKIDir),
			pki.WithControlPlaneEndpoint(constants.LocalControlPlaneEndpoint),
			pki.WithAPIServerCertSANs(params.ApiServerCertSANs),
			pki.WithEncryptionAlgorithmType(pkiconstants.EncryptionAlgorithmType(params.EncryptionAlgorithm)),
			pki.WithCertTreeScheme(certTree),
		)
	}
	return pki.CreatePKIBundle(
		params.NodeName, params.ClusterDomain, ip, params.ServiceSubnetCIDR,
		pki.WithPKIDir(params.PKIDir),
		pki.WithControlPlaneEndpoint(constants.LocalControlPlaneEndpoint),
		pki.WithAPIServerCertSANs(params.ApiServerCertSANs),
		pki.WithEncryptionAlgorithmType(pkiconstants.EncryptionAlgorithmType(params.EncryptionAlgorithm)),
	)
}

func parsePKIParams(pkiDir string, secretData map[string][]byte, node NodeIdentity) PKIParams {
	params := PKIParams{
		NodeName:          node.Name,
		AdvertiseAddress:  node.AdvertiseIP,
		ClusterDomain:     node.ClusterDomain,
		ServiceSubnetCIDR: node.ServiceSubnetCIDR,
		PKIDir:            pkiDir,
	}

	if sans, ok := secretData[constants.SecretKeyCertSANs]; ok && len(sans) > 0 {
		params.ApiServerCertSANs = strings.Split(string(sans), ",")
	}

	if algo, ok := secretData[constants.SecretKeyEncryptionAlgorithm]; ok && len(algo) > 0 {
		params.EncryptionAlgorithm = string(algo)
	}

	return params
}
