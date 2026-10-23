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

package controller

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/naming"
)

func TestPasswordPair(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		obj      any
		expected []string
	}{
		{
			name:     "password with user id",
			obj:      &unstructured.Unstructured{Object: map[string]any{"userID": "jane", "email": "jane@example.com"}},
			expected: []string{naming.OfflineTokenName("jane", naming.LocalConnectorID)},
		},
		{
			name:     "password without user id",
			obj:      &unstructured.Unstructured{Object: map[string]any{"email": "jane@example.com"}},
			expected: nil,
		},
		{
			name:     "typed object",
			obj:      &metav1.PartialObjectMetadata{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			switch obj := tt.obj.(type) {
			case *unstructured.Unstructured:
				got = PasswordPair(obj)
			case *metav1.PartialObjectMetadata:
				got = PasswordPair(obj)
			}
			if !slices.Equal(got, tt.expected) {
				t.Errorf("PasswordPair = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPairIndexes(t *testing.T) {
	t.Parallel()

	pair := naming.OfflineTokenName("jane", "corp-ldap")
	tests := []struct {
		name     string
		extract  func(*unstructured.Unstructured) []string
		object   map[string]any
		expected []string
	}{
		{
			name:     "refresh token",
			extract:  func(u *unstructured.Unstructured) []string { return RefreshTokenPair(u) },
			object:   map[string]any{"connectorID": "corp-ldap", "claims": map[string]any{"userID": "jane"}},
			expected: []string{pair},
		},
		{
			name:     "refresh token without connector",
			extract:  func(u *unstructured.Unstructured) []string { return RefreshTokenPair(u) },
			object:   map[string]any{"claims": map[string]any{"userID": "jane"}},
			expected: nil,
		},
		{
			name:     "auth session",
			extract:  func(u *unstructured.Unstructured) []string { return AuthSessionPair(u) },
			object:   map[string]any{"userID": "jane", "connectorID": "corp-ldap"},
			expected: []string{pair},
		},
		{
			name:     "auth session without user",
			extract:  func(u *unstructured.Unstructured) []string { return AuthSessionPair(u) },
			object:   map[string]any{"connectorID": "corp-ldap"},
			expected: nil,
		},
		{
			name:     "revocation clients",
			extract:  func(u *unstructured.Unstructured) []string { return RevocationClients(u) },
			object:   map[string]any{"clients": []any{"console", "kubeconfig-generator-0"}},
			expected: []string{"console", "kubeconfig-generator-0"},
		},
		{
			name:     "oauth2 client id",
			extract:  func(u *unstructured.Unstructured) []string { return OAuth2ClientID(u) },
			object:   map[string]any{"id": "console-d8-console-dex-authenticator"},
			expected: []string{"console-d8-console-dex-authenticator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.extract(&unstructured.Unstructured{Object: tt.object})
			if !slices.Equal(got, tt.expected) {
				t.Errorf("index = %v, want %v", got, tt.expected)
			}
		})
	}
}
