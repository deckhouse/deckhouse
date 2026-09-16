/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import "testing"

func TestMulticlusterNameFromSecret(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		want       string
	}{
		{
			name:       "namespaced remote secret",
			secretName: "d8-istio/istio-remote-secret-neighbour-0",
			want:       "neighbour-0",
		},
		{
			name:       "bare remote secret name",
			secretName: "istio-remote-secret-neighbour-0",
			want:       "neighbour-0",
		},
		{
			name:       "resource name that itself looks like a remote secret",
			secretName: "d8-istio/istio-remote-secret-istio-remote-secret-x",
			want:       "istio-remote-secret-x",
		},
		{
			name:       "istiod's own cluster reports no secret",
			secretName: "",
			want:       "",
		},
		{
			name:       "a remote secret we didn't render",
			secretName: "other-ns/some-foreign-secret",
			want:       "",
		},
		{
			name:       "our naming, but in a namespace we don't render into",
			secretName: "other-ns/istio-remote-secret-neighbour-0",
			want:       "",
		},
		{
			name:       "a secret in our namespace that isn't a remote secret",
			secretName: "d8-istio/istio-ca-secret",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := multiclusterNameFromSecret(tt.secretName); got != tt.want {
				t.Errorf("multiclusterNameFromSecret(%q) = %q, want %q", tt.secretName, got, tt.want)
			}
		})
	}
}
