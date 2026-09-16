/*
Copyright 2022 YANDEX LLC
Modifications Copyright 2026 Flant JSC

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

package main

import (
	"context"
	"testing"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	"google.golang.org/grpc"
)

type fakeDNSZoneLister struct {
	pages []*dns.ListDnsZonesResponse
	calls int
	err   error
}

func (f *fakeDNSZoneLister) List(_ context.Context, _ *dns.ListDnsZonesRequest, _ ...grpc.CallOption) (*dns.ListDnsZonesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.pages) {
		return &dns.ListDnsZonesResponse{}, nil
	}
	resp := f.pages[f.calls]
	f.calls++
	return resp, nil
}

func TestNormalizeZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "", want: ""},
		{name: "adds trailing dot", in: "example.com", want: "example.com."},
		{name: "keeps trailing dot", in: "example.com.", want: "example.com."},
		{name: "trims spaces", in: "  example.com  ", want: "example.com."},
		{name: "allows hyphen and underscore", in: "foo_bar-baz.example.com", want: "foo_bar-baz.example.com."},
		{name: "rejects quotes", in: `example.com"`, wantErr: true},
		{name: "rejects spaces inside", in: "exam ple.com", wantErr: true},
		{name: "rejects filter injection", in: `evil" OR zone = "x.com`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeZone(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetDNSZone(t *testing.T) {
	t.Parallel()

	public := &dns.DnsZone{
		Id:               "zone-public",
		Zone:             "example.com.",
		PublicVisibility: &dns.PublicVisibility{},
	}
	private := &dns.DnsZone{
		Id:                "zone-private",
		Zone:              "example.com.",
		PrivateVisibility: &dns.PrivateVisibility{},
	}
	otherPublic := &dns.DnsZone{
		Id:               "zone-other",
		Zone:             "other.com.",
		PublicVisibility: &dns.PublicVisibility{},
	}

	t.Run("returns matching public zone", func(t *testing.T) {
		t.Parallel()
		client := &fakeDNSZoneLister{
			pages: []*dns.ListDnsZonesResponse{{
				DnsZones: []*dns.DnsZone{private, otherPublic, public},
			}},
		}
		got, err := getDNSZone(context.Background(), client, "folder", "example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Id != "zone-public" {
			t.Fatalf("got id %q, want zone-public", got.Id)
		}
	})

	t.Run("paginates until match", func(t *testing.T) {
		t.Parallel()
		client := &fakeDNSZoneLister{
			pages: []*dns.ListDnsZonesResponse{
				{DnsZones: []*dns.DnsZone{otherPublic}, NextPageToken: "next"},
				{DnsZones: []*dns.DnsZone{public}},
			},
		}
		got, err := getDNSZone(context.Background(), client, "folder", "example.com.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Id != "zone-public" {
			t.Fatalf("got id %q, want zone-public", got.Id)
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 list calls, got %d", client.calls)
		}
	})

	t.Run("skips private-only match", func(t *testing.T) {
		t.Parallel()
		client := &fakeDNSZoneLister{
			pages: []*dns.ListDnsZonesResponse{{
				DnsZones: []*dns.DnsZone{private},
			}},
		}
		_, err := getDNSZone(context.Background(), client, "folder", "example.com.")
		if err == nil {
			t.Fatal("expected error for private-only zone")
		}
	})

	t.Run("rejects invalid zone name", func(t *testing.T) {
		t.Parallel()
		client := &fakeDNSZoneLister{}
		_, err := getDNSZone(context.Background(), client, "folder", `evil" OR zone = "x`)
		if err == nil {
			t.Fatal("expected invalid zone error")
		}
		if client.calls != 0 {
			t.Fatalf("List must not be called for invalid zone, calls=%d", client.calls)
		}
	})

	t.Run("empty zone", func(t *testing.T) {
		t.Parallel()
		_, err := getDNSZone(context.Background(), &fakeDNSZoneLister{}, "folder", "   ")
		if err == nil {
			t.Fatal("expected empty zone error")
		}
	})
}
