// Copyright 2025 Flant JSC
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

package vsphere

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

// generateTestCAPEM returns a self-signed CA certificate encoded in PEM format.
func generateTestCAPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func newTestVCClient(t *testing.T) *govmomi.Client {
	t.Helper()

	u, err := url.Parse("https://user:pass@vcenter.example.com/sdk")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	soapClient := soap.NewClient(u, false)
	return &govmomi.Client{
		Client: &vim25.Client{Client: soapClient},
	}
}

func TestSetVCClientCA(t *testing.T) {
	validCA := generateTestCAPEM(t)

	t.Run("empty CA bundle leaves transport untouched", func(t *testing.T) {
		vc := newTestVCClient(t)
		before := vc.Transport

		if err := setVCClientCA(vc, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if vc.Transport != before {
			t.Fatalf("transport must not be replaced when CA bundle is empty")
		}
	})

	t.Run("valid CA bundle configures RootCAs", func(t *testing.T) {
		vc := newTestVCClient(t)

		if err := setVCClientCA(vc, validCA); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		transport, ok := vc.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("expected *http.Transport, got %T", vc.Transport)
		}
		if transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
			t.Fatalf("expected RootCAs to be set")
		}
		//nolint:staticcheck // Subjects() is enough to assert the pool is not empty in tests.
		if len(transport.TLSClientConfig.RootCAs.Subjects()) == 0 {
			t.Fatalf("expected at least one CA in the pool")
		}
	})

	t.Run("invalid CA bundle returns error", func(t *testing.T) {
		vc := newTestVCClient(t)

		err := setVCClientCA(vc, "not-a-valid-pem")
		if err == nil {
			t.Fatalf("expected error for invalid CA bundle")
		}
	})
}

func TestSlugKubernetesName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty stays empty", input: "", want: ""},
		{name: "alnum-only label kept as is", input: "datastore1", want: "datastore1"},
		// The dns label regex forbids dashes, so any name containing a dash is slugged
		// with a deterministic murmur hash suffix.
		{name: "name with dashes is slugged", input: "my-datastore-1", want: "my-datastore-1-c7567ed"},
		{name: "spaces are slugged", input: "my datastore", want: "my-datastore-772d4a90"},
		{name: "already dashed name is slugged", input: "dc-cluster-ds", want: "dc-cluster-ds-52483173"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugKubernetesName(tt.input)
			if got != tt.want {
				t.Fatalf("slugKubernetesName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugKubernetesNameDeterministic(t *testing.T) {
	// Names that require slugging must produce a stable, lowercase result.
	input := "Datastore With Spaces And UPPER"

	first := slugKubernetesName(input)
	second := slugKubernetesName(input)
	if first != second {
		t.Fatalf("slug must be deterministic: %q != %q", first, second)
	}
	if first == input {
		t.Fatalf("expected %q to be slugged", input)
	}
}

func TestSlugRespectsMaxSize(t *testing.T) {
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}

	got := slug(long, dnsLabelMaxSize)
	if len(got) > dnsLabelMaxSize {
		t.Fatalf("slug length %d must not exceed max size %d", len(got), dnsLabelMaxSize)
	}
}

func TestShouldNotBeSlugged(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty string should not be slugged", input: "", want: true},
		{name: "valid short label should not be slugged", input: "abc", want: true},
		{name: "string with spaces should be slugged", input: "a b c", want: false},
		{name: "string with dot should be slugged", input: "a.b", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldNotBeSlugged(tt.input, dnsLabelRegex, dnsLabelMaxSize)
			if got != tt.want {
				t.Fatalf("shouldNotBeSlugged(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMurmurHashDeterministic(t *testing.T) {
	a := murmurHash("some", "value")
	b := murmurHash("some", "value")
	if a != b {
		t.Fatalf("murmurHash must be deterministic: %q != %q", a, b)
	}

	c := murmurHash("other", "value")
	if a == c {
		t.Fatalf("murmurHash must differ for different args")
	}
}

func TestIsZoneAllowed(t *testing.T) {
	tests := []struct {
		name    string
		allowed map[string]any
		zone    string
		want    bool
	}{
		{
			name:    "empty allowed list allows any zone",
			allowed: map[string]any{},
			zone:    "zone-a",
			want:    true,
		},
		{
			name:    "zone present in allowed list",
			allowed: map[string]any{"zone-a": struct{}{}, "zone-b": struct{}{}},
			zone:    "zone-a",
			want:    true,
		},
		{
			name:    "zone absent from allowed list",
			allowed: map[string]any{"zone-a": struct{}{}},
			zone:    "zone-c",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isZoneAllowed(tt.allowed, tt.zone)
			if got != tt.want {
				t.Fatalf("isZoneAllowed(%v, %q) = %v, want %v", tt.allowed, tt.zone, got, tt.want)
			}
		})
	}
}
