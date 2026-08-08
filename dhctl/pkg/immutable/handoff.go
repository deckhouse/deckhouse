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
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	// HandoffPort is the port the node's one-shot handoff endpoint listens on.
	HandoffPort = 50001

	// statusPath, handoffPath and collectedPath are the node's bootstrap
	// channel: what it is doing, the credentials, and the confirmation that
	// ends the handover.
	statusPath    = "/bootstrap/status"
	handoffPath   = "/bootstrap/kubeconfig"
	collectedPath = "/bootstrap/collected"

	// handoffCACommonName names the throwaway CA behind the endpoint's
	// certificate.
	handoffCACommonName = "dhctl-immutable-handoff-ca"

	// clusterAdminCommonName and clusterAdminOrganization are what the node
	// signs the installer's request as: the same identity kubeadm gives its
	// admin.conf, so the certificate carries cluster-admin through the usual
	// group binding rather than anything special to this path.
	clusterAdminCommonName   = "kubernetes-admin"
	clusterAdminOrganization = "system:masters"

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

// handoffCacheKey names the handoff material in the dhctl state cache. A second
// bootstrap attempt has to present the same token to a node that already booted
// with the first payload, and verify the certificate that payload carried.
const handoffCacheKey = "immutable-control-plane-handoff"

// collectedKubeconfigCacheKey names the record that the admin kubeconfig is on
// disk, and where. That claim, not "the handover is confirmed", is what a rerun
// needs: the credentials are usable from the moment the file exists.
//
// It is what makes a rerun possible at all. The bootstrap has phases left after
// the handover — installing Deckhouse takes minutes — and nothing skips a phase
// that already completed, so a rerun re-enters the same step. Without this
// record it would go back to a channel that answers 410 at best and, once the
// listener is gone, nothing at all — a refused dial being exactly what a node
// that is still installing itself looks like, so the wait would spend its full
// budget — while the credentials it is looking for sit unread on disk.
//
// Written before ConfirmCollected, never after: that call is what makes the node
// shut the channel for good, so a process that dies between the two — or a cache
// write that fails — must not leave a closed channel and no record of the file.
const collectedKubeconfigCacheKey = "immutable-control-plane-collected-kubeconfig"

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
// generateHandoffMaterial: its private key is never stored and never leaves the
// process. Only the issued certificate travels to the node, and only the
// certificate of the CA is kept, to verify what the node serves with.
type HandoffMaterial struct {
	Token         string `json:"token"`
	CACertPEM     string `json:"caCert"`
	ServerCertPEM string `json:"serverCert"`
	ServerKeyPEM  string `json:"serverKey"`
	// ClientCSRPEM is the request the node signs with the cluster CA, and
	// ClientKeyPEM the key it belongs to. The key never leaves the installer —
	// which is what keeps the channel free of anything worth stealing: what
	// travels back is a certificate, and a certificate is public.
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

	material, err := generateHandoffMaterial(nodeName)
	if err != nil {
		return nil, err
	}

	if err := cache.SaveStruct(ctx, handoffCacheKey, material); err != nil {
		return nil, fmt.Errorf("save the bootstrap handoff credentials to the state cache: %w", err)
	}

	return material, nil
}

// ForgetHandoffClientKey blanks the installer's client key in the cached
// material and keeps everything else it holds.
//
// The key is the one part worth stealing — a private key behind a cluster-admin
// certificate — and it has done its job by the time the handover is confirmed,
// so leaving it in a cache that for dhctl-server is a Secret in the management
// cluster would outlive the bootstrap for nothing.
//
// Dropping the whole entry instead is what must not happen: the state cache
// survives a failed run, so a bootstrap that is rerun after a later phase failed
// would mint fresh material, and a fresh payload renders a different cloudConfig
// tfvar than the one already in the master's user_data. Everything
// handoffPayload() carries therefore stays exactly as the node booted
// with it.
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

// SaveCollectedKubeconfig records where the admin kubeconfig the node served now
// lives.
func SaveCollectedKubeconfig(ctx context.Context, cache state.Cache, adminKubeconfigPath string) error {
	if adminKubeconfigPath == "" {
		return errors.New("record the collected admin kubeconfig: the path is empty")
	}
	if err := cache.Save(ctx, collectedKubeconfigCacheKey, []byte(adminKubeconfigPath)); err != nil {
		return fmt.Errorf("record the collected admin kubeconfig in the state cache: %w", err)
	}
	return nil
}

// LoadCollectedKubeconfig returns the path SaveCollectedKubeconfig stored, or ""
// when no attempt has collected the credentials yet.
func LoadCollectedKubeconfig(ctx context.Context, cache state.Cache) (string, error) {
	inCache, err := cache.InCache(ctx, collectedKubeconfigCacheKey)
	if err != nil {
		return "", fmt.Errorf("look up %s in the state cache: %w", collectedKubeconfigCacheKey, err)
	}
	if !inCache {
		return "", nil
	}

	path, err := cache.Load(ctx, collectedKubeconfigCacheKey)
	if err != nil {
		return "", fmt.Errorf("load %s from the state cache: %w", collectedKubeconfigCacheKey, err)
	}
	return string(path), nil
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

// generateHandoffMaterial mints the token and the TLS material of the handoff
// endpoint.
//
// The certificate is issued for the node's NAME rather than for its address:
// dhctl builds the payload before the VM exists, so no address is known yet,
// and an unverified endpoint would let anything on the path hand dhctl a
// kubeconfig of its own. dhctl dials whatever address the infrastructure
// reports and tells the TLS handshake to check this name instead.
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

	// The key the installer will talk to the cluster with. It stays here; only
	// the request for a certificate travels to the node, and only a certificate
	// comes back. Nothing in the channel is then worth stealing: whoever reads
	// it gets a public certificate for a key they do not have.
	clientKey, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmECDSAP256)
	if err != nil {
		return nil, fmt.Errorf("generate the cluster client key: %w", err)
	}
	clientKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(clientKey)
	if err != nil {
		return nil, fmt.Errorf("encode the cluster client key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: clusterAdminCommonName, Organization: []string{clusterAdminOrganization}},
	}, clientKey)
	if err != nil {
		return nil, fmt.Errorf("build the cluster client certificate request: %w", err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// caKey dies with this call: nothing above stores it, so the only thing
	// that can ever be issued for this channel is the certificate just made.
	return &HandoffMaterial{
		ClientCSRPEM:  string(csrPEM),
		ClientKeyPEM:  string(clientKeyPEM),
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

// Status is what the node reports about its own bootstrap.
type Status struct {
	Phase    Phase  `json:"phase"`
	Message  string `json:"message,omitempty"`
	NodeName string `json:"nodeName,omitempty"`
}

// Phase is what the node says it is doing. A named type rather than bare
// strings: the comparisons live in another package, and between two untyped
// strings any typo compiles and quietly never matches.
type Phase string

// The phases the node reports. PhaseReady means the kubeconfig can be
// collected. PhasePreparing is the only one of the four nothing ever compares
// against — the wait treats anything that is not Ready, Collected or Failed as
// "still working" — and is declared because it is part of the protocol the node
// speaks and appears in the status line the operator reads.
const (
	PhasePreparing Phase = "Preparing"
	PhaseReady     Phase = "Ready"
	PhaseFailed    Phase = "Failed"
	PhaseCollected Phase = "Collected"
)

// FetchStatus asks the node what it is doing. The channel opens when the node
// starts the work, so this answers long before there is a kubeconfig — which is
// the point: without it the installer cannot tell a node that is still
// generating its PKI from one that died on the first image pull.
func FetchStatus(ctx context.Context, in FetchKubeconfigInput) (*Status, error) {
	body, err := in.call(ctx, http.MethodGet, statusPath)
	if err != nil {
		return nil, err
	}
	status := &Status{}
	if err := json.Unmarshal(body, status); err != nil {
		return nil, fmt.Errorf("parse the bootstrap status from the node: %w", err)
	}
	return status, nil
}

// FetchKubeconfig collects the admin kubeconfig from the node. Collecting does
// not end the handover — ConfirmCollected does — so an installer whose
// connection dies mid-read can simply ask again.
func FetchKubeconfig(ctx context.Context, in FetchKubeconfigInput) ([]byte, error) {
	body, err := in.call(ctx, http.MethodGet, handoffPath)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("the handoff endpoint returned an empty admin kubeconfig")
	}
	return body, nil
}

// ConfirmCollected tells the node the kubeconfig is safely in hand. Only this
// ends the handover: until it arrives the node keeps the channel open, because
// a response that reached the socket says nothing about whether the installer
// kept it.
func ConfirmCollected(ctx context.Context, in FetchKubeconfigInput) error {
	_, err := in.call(ctx, http.MethodPost, collectedPath)
	return err
}

// call performs one authenticated request against the node's bootstrap channel.
func (in FetchKubeconfigInput) call(ctx context.Context, method, path string) ([]byte, error) {
	if in.Address == "" || in.ServerName == "" || in.Material == nil {
		return nil, errors.New("reach the node's bootstrap channel: address, server name or handoff material is empty")
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(in.Material.CACertPEM)) {
		return nil, errors.New("the handoff CA certificate PEM holds no certificate")
	}

	request, err := http.NewRequestWithContext(ctx, method, "https://"+in.Address+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build the request to the node's bootstrap channel: %w", err)
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
		return nil, fmt.Errorf("reach the node's bootstrap channel at %s: %w", in.Address, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, handoffResponseLimit))
	if err != nil {
		return nil, fmt.Errorf("read the answer from the node's bootstrap channel: %w", err)
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return body, nil
	case http.StatusUnauthorized:
		return nil, ErrHandoffUnauthorized
	case http.StatusGone:
		return nil, ErrHandoffAlreadyServed
	default:
		return nil, fmt.Errorf("the node's bootstrap channel answered %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
}
