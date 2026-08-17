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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// TestSchemeKnowsWhatThisProcessReads asserts the scheme this binary actually builds.
//
// A scheme missing a kind does not fail at startup. It fails on the first read of that kind,
// with "no kind is registered for the type v1.Secret" — a message about the client library
// that says nothing about the credentials it was trying to fetch or about what stops working
// as a result. Secrets are here because the resources this process reads name their
// credentials instead of carrying them.
//
// Asserted against the production builder on purpose: every test in this module builds its own
// scheme with whatever it needs, so none of them can notice that this one is short. That is
// how a one-line omission reached a cluster and left the registry crash-looping on a config
// file that was never going to be written.
func TestSchemeKnowsWhatThisProcessReads(t *testing.T) {
	scheme, err := buildScheme()
	require.NoError(t, err)

	assert.True(t, scheme.Recognizes(corev1.SchemeGroupVersion.WithKind("Secret")),
		"credentials are read from Secrets, so this scheme has to know them")
	assert.True(t, scheme.Recognizes(registryv1alpha1.GroupVersion.WithKind("RegistryNode")),
		"the module's own kinds must stay registered")
	assert.True(t, scheme.Recognizes(registryv1alpha1.GroupVersion.WithKind("RegistryStorage")))
}
