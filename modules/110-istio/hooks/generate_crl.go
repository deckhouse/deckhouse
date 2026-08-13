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
	// After generate_ca (Order 10): auto-dummy CRL is signed by internal.ca.
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
// Branching:
//  1. Non-empty settings.crl — operator override (incident revoke / custom PEM).
//  2. Else keep existing cacerts ca-crl.pem when it still verifies under the
//     current internal CA (stable across reconciles; avoids churn and accidental
//     wipe of a previously published revoke list when settings.crl is removed).
//  3. Else mint an empty dummy CRL signed by internal.ca so ztunnel gets
//     CRL_PATH/mount from day one and later revokes are hot-reloads only.
func publishCRL(_ context.Context, input *go_hook.HookInput) error {
	if input.Values.Exists(crlConfigPath) {
		crl := input.Values.Get(crlConfigPath).String()
		if strings.TrimSpace(crl) != "" {
			input.Values.Set(internalCRLPath, crl)
			return nil
		}
	}

	certPEM := input.Values.Get(internalCACertPath).String()
	keyPEM := input.Values.Get(internalCAKeyPath).String()
	if strings.TrimSpace(certPEM) == "" || strings.TrimSpace(keyPEM) == "" {
		input.Values.Remove(internalCRLPath)
		return nil
	}

	cert, signer, err := parseCACertAndKey(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("cannot use istio.internal.ca to publish CRL: %w", err)
	}

	if existing := existingCacertsCRL(input); existing != "" {
		if crlStillValidForCA(existing, cert) {
			input.Values.Set(internalCRLPath, existing)
			return nil
		}
	}

	dummy, err := createEmptyCRL(cert, signer)
	if err != nil {
		return fmt.Errorf("cannot generate empty dummy CRL: %w", err)
	}
	input.Values.Set(internalCRLPath, dummy)
	return nil
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

func crlStillValidForCA(crlPEM string, issuer *x509.Certificate) bool {
	found := false
	for _, block := range decodePEMBlocks([]byte(crlPEM)) {
		if block.Type != "X509 CRL" {
			continue
		}
		crl, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			return false
		}
		if err := crl.CheckSignatureFrom(issuer); err != nil {
			return false
		}
		found = true
	}
	return found
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
