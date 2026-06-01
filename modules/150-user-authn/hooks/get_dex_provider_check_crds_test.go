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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestExecuteDexProviderCheckFailsWhenProviderIsMissing(t *testing.T) {
	status := executeDexProviderCheck(
		context.Background(),
		nil,
		nil,
		DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "missing"}},
		DexProviderForCheck{},
	)

	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("expected failed phase, got %q", status.Phase)
	}
	if len(status.Checks) != 1 || status.Checks[0].Name != "providerExists" || status.Checks[0].Status != dexProviderCheckStepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestExecuteDexProviderCheckFailsWhenProviderIsDisabled(t *testing.T) {
	status := executeDexProviderCheck(
		context.Background(),
		nil,
		nil,
		DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "github"}},
		DexProviderForCheck{
			ObjectMeta: metav1.ObjectMeta{Name: "github", Generation: 42},
			Spec: DexProviderForCheckSpec{
				Enabled: ptr.To(false),
				Type:    "Github",
			},
		},
	)

	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("expected failed phase, got %q", status.Phase)
	}
	if status.ObservedDexProviderGeneration != 42 {
		t.Fatalf("expected observed generation 42, got %d", status.ObservedDexProviderGeneration)
	}
	if len(status.Checks) != 2 || status.Checks[1].Name != "providerEnabled" || status.Checks[1].Status != dexProviderCheckStepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestLDAPAddressDefaultsPortFromTLSMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  DexProviderLDAPForCheck
		want string
	}{
		{
			name: "ldaps default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com"},
			want: "ldap.example.com:636",
		},
		{
			name: "plain ldap default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com", InsecureNoSSL: true},
			want: "ldap.example.com:389",
		},
		{
			name: "starttls default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com", StartTLS: true},
			want: "ldap.example.com:389",
		},
		{
			name: "explicit port",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com:1636"},
			want: "ldap.example.com:1636",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := ldapAddress(&tt.cfg)
			if err != nil {
				t.Fatalf("ldapAddress returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
