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

// Package mock builds the cluster objects that decide which cloud provider a NodeGroup resolves
// to, so a test states the provider it wants instead of restating how one is published. Getting
// that restatement wrong (an unlabelled registration, a missing cluster configuration) resolves
// every NodeGroup to no provider, and the test then passes or fails for a reason it never named.
//
// The tests of cloudprovider itself cannot use this: it imports that package.
package mock

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/node-controller/internal/cloudprovider"
)

// Registration is the Secret a provider module publishes
// (modules/030-cloud-provider-*/templates/registration.yaml). The label is the only way
// cloudprovider.GetCatalog finds it, so an unlabelled Secret is not a published provider at all.
func Registration(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cloudprovider.RegistrationSecretNamespace,
			Name:      name,
			Labels:    map[string]string{cloudprovider.RegistrationSecretLabel: ""},
		},
		Data: data,
	}
}

// DefaultRegistration is the registration a single-provider cluster publishes, under the base name.
func DefaultRegistration(data map[string][]byte) *corev1.Secret {
	return Registration(cloudprovider.RegistrationSecretBaseName, data)
}

// InstanceClassRegistration publishes nothing but the InstanceClass contract: the kind a NodeGroup
// may reference and the version it is pinned to. An empty apiVersion is a provider that has not
// published one yet, which callers must wait on rather than guess.
func InstanceClassRegistration(name, kind, apiVersion string) *corev1.Secret {
	return Registration(name, map[string][]byte{
		cloudprovider.InstanceClassKindKey:       []byte(kind),
		cloudprovider.InstanceClassAPIVersionKey: []byte(apiVersion),
	})
}
