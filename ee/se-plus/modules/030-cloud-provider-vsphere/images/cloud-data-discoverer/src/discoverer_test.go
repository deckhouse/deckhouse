/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

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
	"reflect"
	"testing"
	"time"

	"github.com/vmware/govmomi/vim25/soap"

	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/vsphere"
	"github.com/deckhouse/deckhouse/pkg/log"
)

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

func newTestSoapClient(t *testing.T) *soap.Client {
	t.Helper()

	u, err := url.Parse("https://user:pass@vcenter.example.com/sdk")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	return soap.NewClient(u, false)
}

func TestSetCABundleIfNeed(t *testing.T) {
	validCA := generateTestCAPEM(t)

	t.Run("empty CA bundle leaves transport untouched", func(t *testing.T) {
		client := newTestSoapClient(t)
		before := client.Transport

		if err := setCABundleIfNeed(log.NewNop(), client, false, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.Transport != before {
			t.Fatalf("transport must not be replaced when CA bundle is empty")
		}
	})

	t.Run("valid CA bundle configures RootCAs", func(t *testing.T) {
		client := newTestSoapClient(t)

		if err := setCABundleIfNeed(log.NewNop(), client, false, validCA); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("expected *http.Transport, got %T", client.Transport)
		}
		if transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
			t.Fatalf("expected RootCAs to be set")
		}
	})

	t.Run("invalid CA bundle returns error", func(t *testing.T) {
		client := newTestSoapClient(t)

		if err := setCABundleIfNeed(log.NewNop(), client, false, "garbage"); err == nil {
			t.Fatalf("expected error for invalid CA bundle")
		}
	})
}

func TestMergeZones(t *testing.T) {
	tests := []struct {
		name       string
		discovered []string
		fresh      []string
		want       []string
	}{
		{
			name:       "merge with dedup and sort",
			discovered: []string{"zone-b", "zone-a"},
			fresh:      []string{"zone-a", "zone-c"},
			want:       []string{"zone-a", "zone-b", "zone-c"},
		},
		{
			name:       "empty discovered keeps only fresh",
			discovered: nil,
			fresh:      []string{"z2", "z1"},
			want:       []string{"z1", "z2"},
		},
		{
			name:       "empty fresh keeps only discovered",
			discovered: []string{"z1"},
			fresh:      nil,
			want:       []string{"z1"},
		},
		{
			name:       "both empty yields empty slice",
			discovered: nil,
			fresh:      nil,
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeZones(tt.discovered, tt.fresh)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeZones() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeDatastores(t *testing.T) {
	discovered := []v1.VsphereDatastore{
		{Name: "ds-b", DatastoreType: "Datastore"},
	}
	fresh := []vsphere.ZonedDataStore{
		{Name: "ds-a", DatastoreType: "Datastore", Zones: []string{"z1"}},
		// Duplicate name of an already discovered datastore must be dropped.
		{Name: "ds-b", DatastoreType: "DatastoreCluster"},
	}

	got := mergeDatastores(discovered, fresh)

	if len(got) != 2 {
		t.Fatalf("expected 2 datastores after dedup, got %d: %+v", len(got), got)
	}
	// Result must be sorted by name.
	if got[0].Name != "ds-a" || got[1].Name != "ds-b" {
		t.Fatalf("expected sorted by name [ds-a ds-b], got [%s %s]", got[0].Name, got[1].Name)
	}
	// The pre-existing ds-b must win over the fresh duplicate (its type stays Datastore).
	if got[1].DatastoreType != "Datastore" {
		t.Fatalf("expected discovered ds-b to be preserved, got type %q", got[1].DatastoreType)
	}
}

func TestVsphereZonedDataStoresToV1(t *testing.T) {
	in := []vsphere.ZonedDataStore{
		{
			Zones:         []string{"z1", "z2"},
			InventoryPath: "/dc/ds/path",
			Name:          "ds-1",
			DatastoreType: "Datastore",
			DatastoreURL:  "ds:///vmfs/volumes/hash/",
		},
	}

	got := vsphereZonedDataStoresToV1(in)

	want := []v1.VsphereDatastore{
		{
			Zones:         []string{"z1", "z2"},
			InventoryPath: "/dc/ds/path",
			Name:          "ds-1",
			DatastoreType: "Datastore",
			DatastoreURL:  "ds:///vmfs/volumes/hash/",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vsphereZonedDataStoresToV1() = %+v, want %+v", got, want)
	}
}

func TestVsphereZonedDataStoresToV1Empty(t *testing.T) {
	got := vsphereZonedDataStoresToV1(nil)
	if got == nil {
		t.Fatalf("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(got))
	}
}
