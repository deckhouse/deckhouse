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

package cloudprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsRegistration is the one definition of a registration: the watch predicates, the lazy
// InstanceClass source, RegistrationRequests and Load all resolve through it, so every condition it
// checks decides both what is watched and what is loaded.
func TestIsRegistration(t *testing.T) {
	secret := func(namespace, name string, labels map[string]string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: name, Labels: labels,
		}}
	}
	labelled := map[string]string{SecretLabel: ""}

	tests := []struct {
		name string
		obj  *corev1.Secret
		want bool
	}{
		{
			name: "the copy under the bare prefix",
			obj:  secret(SecretNamespace, SecretNamePrefix, labelled),
			want: true,
		},
		{
			name: "the per-provider copy",
			obj:  secret(SecretNamespace, SecretNamePrefix+"-yandex", labelled),
			want: true,
		},
		{
			// The label alone is not enough: it is an empty-valued label anyone can copy, and a
			// Secret outside the prefix is not something a provider module publishes.
			name: "labelled, but named outside the prefix",
			obj:  secret(SecretNamespace, "some-other-secret", labelled),
		},
		{
			name: "named with the prefix, but not labelled",
			obj:  secret(SecretNamespace, SecretNamePrefix+"-aws", nil),
		},
		{
			// Registrations live in one namespace; the same name elsewhere is somebody else's.
			name: "right name and label, wrong namespace",
			obj:  secret("default", SecretNamePrefix, labelled),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsRegistration(tc.obj))
		})
	}
}
