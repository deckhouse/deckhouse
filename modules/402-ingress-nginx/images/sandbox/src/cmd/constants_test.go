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
	"slices"
	"testing"
)

func TestGetSandboxExtraReadIncludesValidationChrootAndConfig(t *testing.T) {
	configPath := "/tmp/nginx/nginx-cfg123"
	got := getSandboxExtraRead(configPath)

	for _, want := range []string{
		"/validation-chroot/*",
		"/usr/local/nginx/sbin/nginx",
		configPath,
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected extra read set to contain %q, got %v", want, got)
		}
	}
}

func TestSandboxSyscallPolicyKeepsValidationSocketOpsButBlocksNetworkDialing(t *testing.T) {
	for _, want := range []string{"socket", "bind", "listen", "setsockopt"} {
		if !slices.Contains(sandboxExtraAllowSyscalls, want) {
			t.Fatalf("expected allowlist to contain %q, got %v", want, sandboxExtraAllowSyscalls)
		}
	}

	for _, denied := range []string{"connect", "sendto", "sendmsg", "recvfrom", "recvmsg"} {
		if slices.Contains(sandboxExtraAllowSyscalls, denied) {
			t.Fatalf("expected allowlist not to contain %q, got %v", denied, sandboxExtraAllowSyscalls)
		}
	}
}
