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

package registry

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/deckhouse/delivery-kit-sdk/pkg/signature"
	ddk "github.com/deckhouse/delivery-kit-sdk/pkg/signature/image"
	"github.com/deckhouse/delivery-kit-sdk/pkg/signver"
	"github.com/deckhouse/rootca"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	sigstore "github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
)

type testSigner struct {
	rootBase64 string
	signer     *signver.SignerVerifier
}

func newTestCertificate(t *testing.T, template, parent *x509.Certificate, public *ecdsa.PublicKey, parentKey *ecdsa.PrivateKey) (*x509.Certificate, []byte) {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, template, parent, public, parentKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func newTestSigner(t *testing.T) testSigner {
	t.Helper()

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	root, rootPEM := newTestCertificate(t, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	_, leafPEM := newTestCertificate(t, leafTemplate, root, &leafKey.PublicKey, rootKey)

	verifier, err := sigstore.LoadECDSASignerVerifier(leafKey, crypto.SHA256)
	require.NoError(t, err)

	return testSigner{
		rootBase64: base64.StdEncoding.EncodeToString(rootPEM),
		signer:     &signver.SignerVerifier{Cert: leafPEM, SignerVerifier: verifier},
	}
}

func (s testSigner) sign(t *testing.T, image v1.Image) v1.Image {
	t.Helper()

	manifest, err := image.Manifest()
	require.NoError(t, err)
	annotations, err := ddk.GetSignatureAnnotationsForImageManifest(t.Context(), s.signer, manifest)
	require.NoError(t, err)

	signed, ok := mutate.Annotations(image, annotations).(v1.Image)
	require.True(t, ok)

	return signed
}

func TestDefaultClient_GetPackageSignCheck(t *testing.T) {
	trusted := newTestSigner(t)
	untrusted := newTestSigner(t)

	unsigned := mustRandomImage(t)
	signedByTrusted := trusted.sign(t, unsigned)
	signedByUntrusted := untrusted.sign(t, unsigned)

	tests := []struct {
		name      string
		image     v1.Image
		signCheck bool
		wantErr   error
	}{
		{name: "signed by a trusted root", image: signedByTrusted, signCheck: true},
		{name: "unsigned", image: unsigned, signCheck: true, wantErr: signature.ErrNoSignature},
		{name: "signed by another root", image: signedByUntrusted, signCheck: true, wantErr: ErrSignatureVerification},
		{name: "unsigned without the check", image: unsigned, signCheck: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newTestRegistry(t)
			digest := pushImage(t, host, tt.image, "deckhouse/package", "v1")
			client := &DefaultClient{signatureRoots: []string{trusted.rootBase64}}
			config := &ClientConfig{Repository: host + "/deckhouse", Scheme: "http", SignCheck: tt.signCheck}

			size, _, reader, err := client.GetPackage(t.Context(), testLogger{}, config, digest, "package")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, ErrSignatureVerification)
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, reader)

				return
			}

			require.NoError(t, err)
			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			require.Equal(t, size, int64(len(body)))
		})
	}
}

func TestDefaultClient_GetPackageTrustsFlantRootByDefault(t *testing.T) {
	require.Equal(t, []string{rootca.RootCABase64}, (&DefaultClient{}).trustedRoots())

	signed := newTestSigner(t).sign(t, mustRandomImage(t))
	host := newTestRegistry(t)
	digest := pushImage(t, host, signed, "deckhouse/package", "v1")
	config := &ClientConfig{Repository: host + "/deckhouse", Scheme: "http", SignCheck: true}

	_, _, reader, err := (&DefaultClient{}).GetPackage(t.Context(), testLogger{}, config, digest, "package")
	require.ErrorIs(t, err, ErrSignatureVerification)
	require.Nil(t, reader)
}

func mustRandomImage(t *testing.T) v1.Image {
	t.Helper()

	image, err := random.Image(256, 1)
	require.NoError(t, err)

	return image
}
