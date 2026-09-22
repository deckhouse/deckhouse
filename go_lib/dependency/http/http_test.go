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

package http

import (
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestIsRestrictedIP(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		restricted bool
	}{
		{name: "public IPv4", ip: "8.8.8.8", restricted: false},
		{name: "public IPv6", ip: "2606:4700:4700::1111", restricted: false},
		{name: "private IPv4", ip: "10.0.0.1", restricted: false},
		{name: "mapped private IPv4", ip: "::ffff:10.0.0.1", restricted: false},
		{name: "RFC1918 192.168", ip: "192.168.1.1", restricted: false},
		{name: "carrier-grade NAT", ip: "100.64.0.1", restricted: false},
		{name: "unique local IPv6", ip: "fd00::1", restricted: false},
		{name: "loopback IPv4", ip: "127.0.0.1", restricted: true},
		{name: "mapped loopback IPv4", ip: "::ffff:127.0.0.1", restricted: true},
		{name: "link-local metadata", ip: "169.254.169.254", restricted: true},
		{name: "loopback IPv6", ip: "::1", restricted: true},
		{name: "link-local IPv6", ip: "fe80::1", restricted: true},
		{name: "unspecified IPv4", ip: "0.0.0.0", restricted: true},
		{name: "multicast IPv4", ip: "224.0.0.1", restricted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRestrictedIP(netip.MustParseAddr(tt.ip)); got != tt.restricted {
				t.Fatalf("isRestrictedIP(%q) = %t, want %t", tt.ip, got, tt.restricted)
			}
		})
	}
}

func TestRestrictedClientDisablesRedirects(t *testing.T) {
	client, ok := NewClient(WithRestrictedNetworkAccess()).(*http.Client)
	if !ok {
		t.Fatal("NewClient returned an unexpected client implementation")
	}
	if client.CheckRedirect == nil {
		t.Fatal("restricted client must define CheckRedirect")
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Fatal("restricted client must reject redirects")
	}
}

func TestRestrictedClientRejectsLiteralLoopbackAddress(t *testing.T) {
	client := NewClient(WithRestrictedNetworkAccess()).(*http.Client)

	_, err := client.Get("https://127.0.0.1:8443")
	if err == nil {
		t.Fatal("restricted client must reject a loopback destination")
	}
	if !strings.Contains(err.Error(), "restricted IP address") {
		t.Fatalf("unexpected error: %v", err)
	}
}
