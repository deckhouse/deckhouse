// Copyright 2026 Flant JSC
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

package immutable

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

const (
	// adminCommonName and adminOrganization authenticate dhctl against the API
	// server the node brings up. system:masters bypasses RBAC, which is what
	// the installer needs before any role binding exists.
	adminCommonName   = "dhctl-bootstrap"
	adminOrganization = "system:masters"

	kubeconfigClusterName = "immutable-bootstrap"
)

// AdminKubeconfigInput is everything BuildAdminKubeconfig needs.
type AdminKubeconfigInput struct {
	// CACertPEM and CAKeyPEM are the cluster CA from the payload dhctl handed
	// the node; the node issued its own serving certificate from the same CA.
	CACertPEM string
	CAKeyPEM  string
	// Server is the API server URL dhctl reaches, either the master directly or
	// the local end of a tunnel through the bastion.
	Server string
}

// BuildAdminKubeconfig mints a client certificate from the cluster CA and
// returns a kubeconfig for it. dhctl needs its own credentials because the
// immutable node keeps the kubeconfigs it generates to itself: there is no
// sshd to fetch admin.conf over.
func BuildAdminKubeconfig(ctx context.Context, in AdminKubeconfigInput) ([]byte, error) {
	if in.CACertPEM == "" || in.CAKeyPEM == "" {
		return nil, fmt.Errorf("build admin kubeconfig: CA certificate or key is empty")
	}
	if in.Server == "" {
		return nil, fmt.Errorf("build admin kubeconfig: server URL is empty")
	}

	caCerts, err := certutil.ParseCertsPEM([]byte(in.CACertPEM))
	if err != nil {
		return nil, fmt.Errorf("parse the cluster CA certificate: %w", err)
	}
	if len(caCerts) == 0 {
		return nil, fmt.Errorf("the cluster CA certificate PEM holds no certificate")
	}

	parsedKey, err := keyutil.ParsePrivateKeyPEM([]byte(in.CAKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse the cluster CA key: %w", err)
	}
	caKey, ok := parsedKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("the cluster CA key of type %T cannot sign", parsedKey)
	}

	cert, key, err := pkiutil.NewCertAndKey(caCerts[0], caKey, pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   adminCommonName,
			Organization: []string{adminOrganization},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		EncryptionAlgorithm:       pkiutil.DetectEncryptionAlgorithm(caCerts[0]),
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("issue the installer client certificate: %w", err)
	}

	keyPEM, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("encode the installer client key: %w", err)
	}

	kubeconfig := clientcmdapi.NewConfig()
	kubeconfig.Clusters[kubeconfigClusterName] = &clientcmdapi.Cluster{
		Server:                   in.Server,
		CertificateAuthorityData: []byte(in.CACertPEM),
	}
	kubeconfig.AuthInfos[adminCommonName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: pkiutil.EncodeCertificate(cert),
		ClientKeyData:         keyPEM,
	}
	kubeconfig.Contexts[kubeconfigClusterName] = &clientcmdapi.Context{
		Cluster:  kubeconfigClusterName,
		AuthInfo: adminCommonName,
	}
	kubeconfig.CurrentContext = kubeconfigClusterName

	content, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the installer kubeconfig: %w", err)
	}

	return content, nil
}
