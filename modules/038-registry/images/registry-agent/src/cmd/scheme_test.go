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
	"io"
	"log/slog"
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

// TestTheSourceIsWiredToResolveCredentials asserts the construction main uses, not one a test
// assembled to its own liking.
//
// A Source without a resolver does not fail. It refuses every layout that names its
// credentials, treats the refusal as an unreachable API server, serves the layout the node was
// installed with instead, and reports success — so the node keeps pulling while nothing the
// cluster configures ever reaches it. That is what happened, and every test in the layout
// package passed throughout, because each of them wires the resolver it is testing.
func TestTheSourceIsWiredToResolveCredentials(t *testing.T) {
	source := buildSource(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		options{nodeName: "worker-1", cachePath: "/tmp/layout.json", bootstrap: "/tmp/bootstrap.json"},
	)

	require.NotNil(t, source.Resolver, "without this the agent can never apply what the cluster gives it")
	assert.Equal(t, moduleNamespace, source.Resolver.Namespace)

	// And the same requirement stated where it stops the process rather than degrading it.
	assert.NoError(t, source.Validate())

	source.Resolver = nil
	assert.Error(t, source.Validate(),
		"a Source that cannot resolve credentials has to be refused, not quietly downgraded")
}
