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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	// HandoffPort is the port the node's one-shot handoff endpoint listens on.
	HandoffPort = 50001

	// handoffPath is the endpoint the admin kubeconfig is served from.
	handoffPath = "/bootstrap/kubeconfig"

	// handoffCACommonName names the throwaway CA behind the endpoint's
	// certificate.
	handoffCACommonName = "dhctl-immutable-handoff-ca"

	// handoffTokenBytes is the size of the bearer token that authorises the one
	// collection.
	handoffTokenBytes = 32

	// handoffRequestTimeout bounds a single collection attempt. The body is a
	// few kilobytes; anything slower is a node that is not ready.
	handoffRequestTimeout = 30 * time.Second

	// handoffResponseLimit caps what is read from the endpoint. A kubeconfig is
	// a few kilobytes and dhctl parses whatever comes back.
	handoffResponseLimit = 1 << 20
)

// HandoffCacheKey names the handoff material in the dhctl state cache. A second
// bootstrap attempt has to present the same token to a node that already booted
// with the first payload, and verify the certificate that payload carried.
const HandoffCacheKey = "immutable-control-plane-handoff"

var (
	// ErrHandoffUnauthorized means the node rejected the token. dhctl and the
	// node hold different payloads, and no amount of waiting fixes that.
	ErrHandoffUnauthorized = errors.New(
		"the handoff endpoint rejected the bootstrap token: the master booted with a payload other than the one in the state cache",
	)

	// ErrHandoffAlreadyServed means the credentials have been collected before.
	// The endpoint serves once and then closes; there is nothing left to wait
	// for.
	ErrHandoffAlreadyServed = errors.New(
		"the handoff endpoint has already handed the cluster credentials over: it serves once and is closed for good",
	)
)

// HandoffMaterial is dhctl's side of the one-shot channel.
//
// The CA that signed ServerCertPEM is generated, used and dropped inside
// NewHandoffMaterial: its private key is never stored and never leaves the
// process. Only the issued certificate travels to the node, and only the
// certificate of the CA is kept, to verify what the node serves with.
type HandoffMaterial struct {
	Token         string `json:"token"`
	CACertPEM     string `json:"caCert"`
	ServerCertPEM string `json:"serverCert"`
	ServerKeyPEM  string `json:"serverKey"`
}

// Payload is the part of the material the node is given.
func (m HandoffMaterial) Payload() Handoff {
	return Handoff{
		Token:      m.Token,
		ServerCert: m.ServerCertPEM,
		ServerKey:  m.ServerKeyPEM,
	}
}

// HandoffMaterialFor returns the material of the running bootstrap, minting it
// on the first call and reusing the cached one afterwards. A second attempt
// must present the token the node already booted with and verify the
// certificate that payload carried, so fresh material would lock dhctl out of
// the node it created.
func HandoffMaterialFor(ctx context.Context, cache state.Cache, nodeName string) (*HandoffMaterial, error) {
	cached, err := LoadHandoffMaterial(ctx, cache)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		dhlog.FromContext(ctx).InfoContext(ctx, "Reusing the bootstrap handoff credentials from the state cache")
		return cached, nil
	}

	material, err := NewHandoffMaterial(nodeName)
	if err != nil {
		return nil, err
	}

	if err := cache.SaveStruct(ctx, HandoffCacheKey, material); err != nil {
		return nil, fmt.Errorf("save the bootstrap handoff credentials to the state cache: %w", err)
	}

	return material, nil
}

// LoadHandoffMaterial returns the material a previous HandoffMaterialFor
// stored, or nil when the cache holds none.
func LoadHandoffMaterial(ctx context.Context, cache state.Cache) (*HandoffMaterial, error) {
	inCache, err := cache.InCache(ctx, HandoffCacheKey)
	if err != nil {
		return nil, fmt.Errorf("look up %s in the state cache: %w", HandoffCacheKey, err)
	}
	if !inCache {
		return nil, nil
	}

	material := new(HandoffMaterial)
	if err := cache.LoadStruct(ctx, HandoffCacheKey, material); err != nil {
		return nil, fmt.Errorf("load %s from the state cache: %w", HandoffCacheKey, err)
	}
	if material.Token == "" || material.CACertPEM == "" {
		return nil, fmt.Errorf("%s in the state cache carries no token or CA certificate", HandoffCacheKey)
	}
	return material, nil
}

// NewHandoffMaterial mints the token and the TLS material of the handoff
// endpoint.
//
// The certificate is issued for the node's NAME rather than for its address:
// dhctl builds the payload before the VM exists, so no address is known yet,
// and an unverified endpoint would let anything on the path hand dhctl a
// kubeconfig of its own. dhctl dials whatever address the infrastructure
// reports and tells the TLS handshake to check this name instead.
func NewHandoffMaterial(nodeName string) (*HandoffMaterial, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("generate the handoff credentials: node name is empty")
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

	// caKey dies with this call: nothing above stores it, so the only thing
	// that can ever be issued for this channel is the certificate just made.
	return &HandoffMaterial{
		Token:         base64.RawURLEncoding.EncodeToString(token),
		CACertPEM:     string(pkiutil.EncodeCertificate(caCert)),
		ServerCertPEM: string(pkiutil.EncodeCertificate(serverCert)),
		ServerKeyPEM:  string(serverKeyPEM),
	}, nil
}

// FetchKubeconfigInput is everything FetchKubeconfig needs.
type FetchKubeconfigInput struct {
	// Address is the host:port dhctl reaches the endpoint on, either the master
	// directly or the local end of a tunnel through the bastion.
	Address string
	// ServerName is the name the endpoint's certificate is issued for. It is
	// deliberately not Address: the certificate was issued before the VM
	// existed.
	ServerName string
	// Material carries the CA the endpoint is verified against and the token
	// that authorises the collection.
	Material *HandoffMaterial
}

// FetchKubeconfig collects the admin kubeconfig from the node's handoff
// endpoint. One attempt: the endpoint appears only once the node has installed
// itself and brought a control plane up, so the caller retries.
func FetchKubeconfig(ctx context.Context, in FetchKubeconfigInput) ([]byte, error) {
	if in.Address == "" || in.ServerName == "" || in.Material == nil {
		return nil, fmt.Errorf("fetch the admin kubeconfig: address, server name or handoff material is empty")
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(in.Material.CACertPEM)) {
		return nil, fmt.Errorf("the handoff CA certificate PEM holds no certificate")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+in.Address+handoffPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build the handoff request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+in.Material.Token)

	client := &http.Client{
		Timeout: handoffRequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				ServerName: in.ServerName,
				MinVersion: tls.VersionTLS12,
			},
		},
	}
	defer client.CloseIdleConnections()

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("reach the handoff endpoint at %s: %w", in.Address, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, handoffResponseLimit))
	if err != nil {
		return nil, fmt.Errorf("read the admin kubeconfig from the handoff endpoint: %w", err)
	}

	switch response.StatusCode {
	case http.StatusOK:
		if len(body) == 0 {
			return nil, fmt.Errorf("the handoff endpoint returned an empty admin kubeconfig")
		}
		return body, nil
	case http.StatusUnauthorized:
		return nil, ErrHandoffUnauthorized
	case http.StatusGone:
		return nil, ErrHandoffAlreadyServed
	default:
		return nil, fmt.Errorf("the handoff endpoint answered %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
}
