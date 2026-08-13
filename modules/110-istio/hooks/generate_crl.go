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

package hooks

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/tidwall/gjson"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	crlConfigPath   = "istio.crl"
	internalCRLPath = "istio.internal.crl"
	cacertsCRLKey   = "ca-crl.pem"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// After generate_ca (10) and alliance_metadata_merge (10): local CRL is signed
	// by internal.ca; peer CRLs come from federation/multicluster values.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret_ca_crl",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyCacertsCRLFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cacerts"},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, publishCRL)

func applyCacertsCRLFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert cacerts secret to secret: %v", err)
	}

	return string(secret.Data[cacertsCRLKey]), nil
}

// publishCRL sets istio.internal.crl for rendering ca-crl.pem.
//
// Local part:
//  1. Non-empty settings.crl — operator override.
//  2. Else keep cacerts ca-crl.pem blocks that still verify under internal.ca.
//  3. Else mint an empty dummy CRL signed by internal.ca.
//
// Then append peer CRLs from istio.internal.federations[].crl (federation
// metadata exchange PoC) so federated issuers are covered for mTLS verify when
// CRL enforcement is on. Multicluster is intentionally not merged here.
func publishCRL(_ context.Context, input *go_hook.HookInput) error {
	local, err := resolveLocalCRL(input)
	if err != nil {
		return err
	}
	if local == "" {
		input.Values.Remove(internalCRLPath)
		return nil
	}

	input.Values.Set(internalCRLPath, concatCRLPEMs(local, collectPeerCRLs(input)))
	return nil
}

func resolveLocalCRL(input *go_hook.HookInput) (string, error) {
	if input.Values.Exists(crlConfigPath) {
		crl := input.Values.Get(crlConfigPath).String()
		if strings.TrimSpace(crl) != "" {
			return crl, nil
		}
	}

	certPEM := input.Values.Get(internalCACertPath).String()
	keyPEM := input.Values.Get(internalCAKeyPath).String()
	if strings.TrimSpace(certPEM) == "" || strings.TrimSpace(keyPEM) == "" {
		return "", nil
	}

	cert, signer, err := parseCACertAndKey(certPEM, keyPEM)
	if err != nil {
		return "", fmt.Errorf("cannot use istio.internal.ca to publish CRL: %w", err)
	}

	if existing := existingCacertsCRL(input); existing != "" {
		if local := filterCRLBlocksSignedBy(existing, cert); local != "" {
			return local, nil
		}
	}

	dummy, err := createEmptyCRL(cert, signer)
	if err != nil {
		return "", fmt.Errorf("cannot generate empty dummy CRL: %w", err)
	}
	return dummy, nil
}

func collectPeerCRLs(input *go_hook.HookInput) string {
	var b strings.Builder
	appendPeerCRLArray(&b, input.Values.Get("istio.internal.federations").Array())
	return b.String()
}

func appendPeerCRLArray(b *strings.Builder, peers []gjson.Result) {
	for _, peer := range peers {
		crl := peer.Get("crl").String()
		if strings.TrimSpace(crl) == "" {
			continue
		}
		b.WriteString(strings.TrimSpace(crl))
		b.WriteByte('\n')
	}
}

func concatCRLPEMs(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return b.String()
}

func existingCacertsCRL(input *go_hook.HookInput) string {
	snapshots := input.Snapshots.Get("secret_ca_crl")
	if len(snapshots) != 1 {
		return ""
	}
	var fromSecret string
	if err := snapshots[0].UnmarshalTo(&fromSecret); err != nil {
		return ""
	}
	return fromSecret
}

// filterCRLBlocksSignedBy keeps only X509 CRL PEM blocks verifiable by issuer.
// Used to strip previously concatenated peer CRLs from cacerts before rebuild.
func filterCRLBlocksSignedBy(crlPEM string, issuer *x509.Certificate) string {
	var b strings.Builder
	for _, block := range decodePEMBlocks([]byte(crlPEM)) {
		if block.Type != "X509 CRL" {
			continue
		}
		crl, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			continue
		}
		if err := crl.CheckSignatureFrom(issuer); err != nil {
			continue
		}
		b.Write(pem.EncodeToMemory(block))
	}
	return b.String()
}

func createEmptyCRL(caCert *x509.Certificate, signer crypto.Signer) (string, error) {
	now := time.Now()
	crlDER, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: now.Add(-time.Minute),
		NextUpdate: now.Add(365 * 24 * time.Hour),
	}, caCert, signer)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlDER})), nil
}

func parseCACertAndKey(certPEM, keyPEM string) (*x509.Certificate, crypto.Signer, error) {
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return nil, nil, fmt.Errorf("signing certificate is not valid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse signing certificate: %w", err)
	}

	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("signing key is not valid PEM")
	}

	var parsed any
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "RSA PRIVATE KEY":
		parsed, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		parsed, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	default:
		// cfssl / tls often emit PKCS#8 without a strict type match attempt order.
		parsed, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			if parsed, err = x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
				parsed, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
			}
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parse signing key: %w", err)
	}

	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("signing key type %T does not implement crypto.Signer", parsed)
	}

	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		priv, ok := signer.(*rsa.PrivateKey)
		if !ok || priv.N.Cmp(pub.N) != 0 {
			return nil, nil, fmt.Errorf("signing key does not match certificate")
		}
	case *ecdsa.PublicKey:
		priv, ok := signer.(*ecdsa.PrivateKey)
		if !ok || priv.X.Cmp(pub.X) != 0 || priv.Y.Cmp(pub.Y) != 0 {
			return nil, nil, fmt.Errorf("signing key does not match certificate")
		}
	}

	return cert, signer, nil
}

func decodePEMBlocks(b []byte) []*pem.Block {
	var blocks []*pem.Block
	for {
		var block *pem.Block
		block, b = pem.Decode(b)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
	}
	return blocks
}
