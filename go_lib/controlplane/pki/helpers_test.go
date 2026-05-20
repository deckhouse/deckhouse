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

package pki

import (
	"net"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
)

func TestFirstIPInCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		want    string
		wantErr bool
	}{
		{
			name: "IPv4 /12 service CIDR",
			cidr: "10.96.0.0/12",
			want: "10.96.0.1",
		},
		{
			name: "IPv4 /24 subnet",
			cidr: "192.168.1.0/24",
			want: "192.168.1.1",
		},
		{
			name: "IPv4 /16 subnet",
			cidr: "172.16.0.0/16",
			want: "172.16.0.1",
		},
		{
			name:    "invalid CIDR",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
		{
			name:    "plain IP without prefix length",
			cidr:    "10.0.0.1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip, err := firstIPInCIDR(tc.cidr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, ip.String())
		})
	}
}

func TestRemoveDuplicateAltNames(t *testing.T) {
	t.Run("deduplicates DNS names", func(t *testing.T) {
		altNames := &certutil.AltNames{
			DNSNames: []string{"example.com", "foo.bar", "example.com"},
		}
		pkiutil.RemoveDuplicateAltNames(altNames)
		assert.Equal(t, []string{"example.com", "foo.bar"}, altNames.DNSNames)
	})

	t.Run("deduplicates IP addresses", func(t *testing.T) {
		ip := net.ParseIP("10.0.0.1")
		altNames := &certutil.AltNames{
			IPs: []net.IP{ip, ip, net.ParseIP("10.0.0.2")},
		}
		pkiutil.RemoveDuplicateAltNames(altNames)
		assert.Len(t, altNames.IPs, 2)
	})

	t.Run("nil input is a no-op", func(t *testing.T) {
		// Must not panic.
		pkiutil.RemoveDuplicateAltNames(nil)
	})
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "hostname with port", input: "example.com:6443", want: "example.com"},
		{name: "IPv4 address with port", input: "10.0.0.1:6443", want: "10.0.0.1"},
		{name: "IPv6 address with port", input: "[::1]:6443", want: "::1"},
		{name: "hostname without port", input: "example.com", want: "example.com"},
		{name: "IPv4 address without port", input: "10.0.0.1", want: "10.0.0.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, stripPort(tc.input))
		})
	}
}
