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

package main

import (
	"encoding/binary"
	"net/netip"
	"reflect"
	"slices"
	"syscall"
	"testing"
)

func TestParseSandboxArgsWithEquals(t *testing.T) {
	dnsPolicy, argv, err := parseSandboxArgs([]string{
		"--allow-dns-to=10.96.0.10:53",
		"--",
		"/usr/bin/unshare",
		"-R",
		"/validation-chroot",
	})
	if err != nil {
		t.Fatalf("parseSandboxArgs returned error: %v", err)
	}
	if dnsPolicy == nil || dnsPolicy.server != netip.MustParseAddrPort("10.96.0.10:53") {
		t.Fatalf("unexpected dns policy: %#v", dnsPolicy)
	}

	wantArgv := []string{"/usr/bin/unshare", "-R", "/validation-chroot"}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("unexpected argv, got %v want %v", argv, wantArgv)
	}
}

func TestParseSandboxArgsWithSeparateValue(t *testing.T) {
	dnsPolicy, argv, err := parseSandboxArgs([]string{
		"--allow-dns-to",
		"10.96.0.10:53",
		"/usr/bin/unshare",
	})
	if err != nil {
		t.Fatalf("parseSandboxArgs returned error: %v", err)
	}
	if dnsPolicy == nil || dnsPolicy.server != netip.MustParseAddrPort("10.96.0.10:53") {
		t.Fatalf("unexpected dns policy: %#v", dnsPolicy)
	}

	wantArgv := []string{"/usr/bin/unshare"}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("unexpected argv, got %v want %v", argv, wantArgv)
	}
}

func TestParseSandboxArgsRejectsUnknownFlag(t *testing.T) {
	if _, _, err := parseSandboxArgs([]string{"--bad-flag", "/usr/bin/unshare"}); err == nil {
		t.Fatal("expected parseSandboxArgs to reject unknown flag")
	}
}

func TestParseSockaddrAddrPortIPv4(t *testing.T) {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint16(buf[:2], syscall.AF_INET)
	binary.BigEndian.PutUint16(buf[2:4], 53)
	copy(buf[4:8], []byte{10, 96, 0, 10})

	got, err := parseSockaddrAddrPort(buf)
	if err != nil {
		t.Fatalf("parseSockaddrAddrPort returned error: %v", err)
	}

	want := netip.MustParseAddrPort("10.96.0.10:53")
	if got != want {
		t.Fatalf("unexpected addr:port, got %s want %s", got, want)
	}
}

func TestParseSockaddrAddrPortRejectsUnixSocket(t *testing.T) {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint16(buf[:2], syscall.AF_UNIX)

	if _, err := parseSockaddrAddrPort(buf); err == nil {
		t.Fatal("expected unsupported family error")
	}
}

func TestSandboxExtraTraceSyscallsWithDNSPolicy(t *testing.T) {
	got := sandboxExtraTraceSyscalls(&sandboxDNSPolicy{
		server: netip.MustParseAddrPort("10.96.0.10:53"),
	})

	for _, want := range []string{"connect", "sendmsg", "sendto", "recvmsg", "recvfrom", "close"} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected traced syscalls to include %q, got %v", want, got)
		}
	}
}

func TestSandboxDNSPolicyAllowsConnectDestination(t *testing.T) {
	policy := &sandboxDNSPolicy{
		server: netip.MustParseAddrPort("10.96.0.10:53"),
	}

	for _, allowed := range []string{
		"10.96.0.10:53",
		"127.0.0.1:65535",
		"[::1]:65535",
		"5.255.255.242:65535",
		"[2a02:6b8::2:242]:65535",
	} {
		dst := netip.MustParseAddrPort(allowed)
		if !policy.allowsConnectDestination(dst) {
			t.Fatalf("expected %s to be an allowed connect destination", dst)
		}
	}

	if policy.allowsConnectDestination(netip.MustParseAddrPort("127.0.0.1:53")) {
		t.Fatal("expected 127.0.0.1:53 to remain disallowed")
	}
	if policy.allowsConnectDestination(netip.MustParseAddrPort("5.255.255.242:80")) {
		t.Fatal("expected non-probe port to remain disallowed")
	}
}
