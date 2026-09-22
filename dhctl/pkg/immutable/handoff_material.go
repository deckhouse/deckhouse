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
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	// handoffCACommonName names the throwaway CA behind the endpoint's
	// certificate.
	handoffCACommonName = "dhctl-immutable-handoff-ca"

	// clusterAdminCommonName and clusterAdminOrganization are what the node
	// signs the installer's request as: the same identity as kubeadm's
	// admin.conf, so cluster-admin comes through the usual group binding.
	clusterAdminCommonName   = "kubernetes-admin"
	clusterAdminOrganization = "system:masters"

	// handoffTokenBytes is the size of the bearer token that authorises the one
	// collection.
	handoffTokenBytes = 32
)

// HandoffMaterial is dhctl's side of the one-shot channel. The CA that signed
// ServerCertPEM is dropped inside generateHandoffMaterial: only its certificate
// is kept, to verify what the node serves with; the CA key is never stored.
type HandoffMaterial struct {
	Token         string `json:"token"`
	CACertPEM     string `json:"caCert"`
	ServerCertPEM string `json:"serverCert"`
	ServerKeyPEM  string `json:"serverKey"`
	// ClientCSRPEM is the request the node signs with the cluster CA, and
	// ClientKeyPEM the key it belongs to. The key never leaves the installer;
	// what travels back is a certificate, which is public.
	ClientCSRPEM string `json:"clientCSR"`
	ClientKeyPEM string `json:"clientKey"`
}

// handoffPayload is the part of the material the node is given. Everything it
// reads has to survive ForgetHandoffClientKey untouched: a rerun renders this
// again and the bytes must match what the master already booted with.
func handoffPayload(m HandoffMaterial) handoff {
	return handoff{
		Token:      m.Token,
		ServerCert: m.ServerCertPEM,
		ServerKey:  m.ServerKeyPEM,
		ClientCSR:  m.ClientCSRPEM,
	}
}

// HandoffMaterialFor returns the material of the running bootstrap, minting it
// on the first call and reusing the cached one afterwards: fresh material would
// lock dhctl out of a node that already booted with the first payload.
func HandoffMaterialFor(ctx context.Context, cache state.Cache, nodeName string) (*HandoffMaterial, error) {
	cached, err := LoadHandoffMaterial(ctx, cache)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		dhlog.FromContext(ctx).InfoContext(ctx, "Reusing the bootstrap handoff credentials from the state cache")
		return cached, nil
	}

	material, err := generateHandoffMaterial(nodeName)
	if err != nil {
		return nil, err
	}

	if err := cache.SaveStruct(ctx, handoffCacheKey, material); err != nil {
		return nil, fmt.Errorf("save the bootstrap handoff credentials to the state cache: %w", err)
	}

	return material, nil
}

// ForgetHandoffClientKey blanks the installer's client key — the one part worth
// stealing — in the cached material and keeps the rest: dropping the whole entry
// would make a rerun render a payload other than what the master booted with.
func ForgetHandoffClientKey(ctx context.Context, cache state.Cache) error {
	material, err := LoadHandoffMaterial(ctx, cache)
	if err != nil {
		return err
	}
	if material == nil || material.ClientKeyPEM == "" {
		return nil
	}

	material.ClientKeyPEM = ""
	if err := cache.SaveStruct(ctx, handoffCacheKey, material); err != nil {
		return fmt.Errorf("save the bootstrap handoff credentials to the state cache: %w", err)
	}
	return nil
}

// LoadHandoffMaterial returns the material a previous HandoffMaterialFor
// stored, or nil when the cache holds none.
func LoadHandoffMaterial(ctx context.Context, cache state.Cache) (*HandoffMaterial, error) {
	inCache, err := cache.InCache(ctx, handoffCacheKey)
	if err != nil {
		return nil, fmt.Errorf("look up %s in the state cache: %w", handoffCacheKey, err)
	}
	if !inCache {
		return nil, nil
	}

	material := new(HandoffMaterial)
	if err := cache.LoadStruct(ctx, handoffCacheKey, material); err != nil {
		return nil, fmt.Errorf("load %s from the state cache: %w", handoffCacheKey, err)
	}
	if material.Token == "" || material.CACertPEM == "" {
		return nil, fmt.Errorf("%s in the state cache carries no token or CA certificate", handoffCacheKey)
	}
	return material, nil
}

// generateHandoffMaterial mints the token and TLS material of the handoff
// endpoint. The certificate is issued for the node's NAME — no address exists
// yet — and dhctl tells the TLS handshake to check that name when dialing.
func generateHandoffMaterial(nodeName string) (*HandoffMaterial, error) {
	if nodeName == "" {
		return nil, errors.New("generate the handoff credentials: node name is empty")
	}

	token := make([]byte, handoffTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("generate the handoff token: %w", err)
	}

	caKey, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmECDSAP256)
	if err != nil {
		return nil, fmt.Errorf("generate the handoff CA key: %w", err)
	}

	caCert, err := pkiutil.NewSelfSignedCACert(pkiutil.CertConfig{
		Config:                    certutil.Config{CommonName: handoffCACommonName},
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	}, caKey)
	if err != nil {
		return nil, fmt.Errorf("issue the handoff CA certificate: %w", err)
	}

	serverCert, serverKey, err := pkiutil.NewCertAndKey(caCert, caKey, pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: nodeName,
			AltNames:   certutil.AltNames{DNSNames: []string{nodeName}},
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		EncryptionAlgorithm:       constants.EncryptionAlgorithmECDSAP256,
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("issue the handoff serving certificate: %w", err)
	}

	serverKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(serverKey)
	if err != nil {
		return nil, fmt.Errorf("encode the handoff serving key: %w", err)
	}

	clientKeyPEM, csrPEM, err := generateClientKeyAndCSR()
	if err != nil {
		return nil, err
	}

	// caKey dies with this call: nothing above stores it, so the only thing
	// that can ever be issued for this channel is the certificate just made.
	return &HandoffMaterial{
		ClientCSRPEM:  csrPEM,
		ClientKeyPEM:  clientKeyPEM,
		Token:         base64.RawURLEncoding.EncodeToString(token),
		CACertPEM:     string(pkiutil.EncodeCertificate(caCert)),
		ServerCertPEM: string(pkiutil.EncodeCertificate(serverCert)),
		ServerKeyPEM:  string(serverKeyPEM),
	}, nil
}

// generateClientKeyAndCSR mints the key the installer will talk to the cluster
// with, and the request the node signs with the cluster CA. The key stays here;
// only the CSR travels and only a public certificate comes back.
func generateClientKeyAndCSR() (string, string, error) {
	clientKey, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmECDSAP256)
	if err != nil {
		return "", "", fmt.Errorf("generate the cluster client key: %w", err)
	}
	clientKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(clientKey)
	if err != nil {
		return "", "", fmt.Errorf("encode the cluster client key: %w", err)
	}

	csrPEM, err := certutil.MakeCSR(clientKey, &pkix.Name{
		CommonName:   clusterAdminCommonName,
		Organization: []string{clusterAdminOrganization},
	}, nil, nil)
	if err != nil {
		return "", "", fmt.Errorf("build the cluster client certificate request: %w", err)
	}

	return string(clientKeyPEM), string(csrPEM), nil
}
