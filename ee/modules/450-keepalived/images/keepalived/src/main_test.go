/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"strings"
	"testing"
)

func TestParseIPv4CIDR(t *testing.T) {
	addr, prefix, err := parseIPv4CIDR("192.168.42.15/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prefix != 24 {
		t.Fatalf("expected prefix 24, got %d", prefix)
	}
	if addr != 192<<24|168<<16|42<<8|15 {
		t.Fatalf("unexpected address value: %d", addr)
	}

	if _, _, err := parseIPv4CIDR("not-a-cidr"); err == nil {
		t.Fatal("expected error for malformed CIDR")
	}
	if _, _, err := parseIPv4CIDR("10.0.0.1/33"); err == nil {
		t.Fatal("expected error for out-of-range prefix")
	}
}

func TestIsNetworkInNetwork(t *testing.T) {
	cases := []struct {
		net1, net2 string
		want       bool
	}{
		{"192.168.42.15/24", "192.168.42.0/24", true},
		{"192.168.42.15/32", "192.168.42.0/24", true},
		{"192.168.43.15/24", "192.168.42.0/24", false},
		{"10.0.0.5/24", "10.0.0.0/16", true},
	}
	for _, c := range cases {
		got, err := isNetworkInNetwork(c.net1, c.net2)
		if err != nil {
			t.Fatalf("unexpected error for %s in %s: %v", c.net1, c.net2, err)
		}
		if got != c.want {
			t.Errorf("isNetworkInNetwork(%q, %q) = %v, want %v", c.net1, c.net2, got, c.want)
		}
	}
}

func TestPodNumberFromEnv(t *testing.T) {
	t.Setenv("POD_NAME", "keepalived-main-2")
	got, err := podNumberFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}

	t.Setenv("POD_NAME", "not-a-number-")
	if _, err := podNumberFromEnv(); err == nil {
		t.Fatal("expected error for non-numeric pod suffix")
	}
}

func TestRenderVRRPInstancePreempt(t *testing.T) {
	noPreempt := false
	instance := vrrpInstance{
		ID:      5,
		Preempt: &noPreempt,
		Interface: interfaceSpec{
			DetectionStrategy: "Name",
			Name:              "eth0",
		},
		VirtualIPAddresses: []virtualIP{
			{Address: "192.168.42.15/32"},
		},
	}

	rendered, err := renderVRRPInstance(instance, 0, 0, 1, "secret", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(rendered, "vrrp_instance instance_5 {") {
		t.Errorf("rendered config missing instance name: %s", rendered)
	}
	if !strings.Contains(rendered, "\n  nopreempt\n") {
		t.Errorf("rendered config should contain nopreempt when Preempt is false: %s", rendered)
	}
	if !strings.Contains(rendered, "    192.168.42.15/32 dev eth0") {
		t.Errorf("rendered config missing virtual IP line: %s", rendered)
	}
}

func TestRenderVRRPInstanceUnknownInterface(t *testing.T) {
	instance := vrrpInstance{
		ID: 1,
		Interface: interfaceSpec{
			DetectionStrategy: "Bogus",
		},
	}
	if _, err := renderVRRPInstance(instance, 0, 0, 1, "secret", nil); err == nil {
		t.Fatal("expected error for unknown detection strategy")
	}
}
