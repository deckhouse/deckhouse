// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Anti-duplication tests for the option pipeline.
//
// The client bakes opts.Auth / opts.Keychain / opts.UserAgent / transport
// into a []remote.Option slice exactly once at construction time
// (NewClientWithOptions → buildRemoteOptions). Every request method then
// does `append([]remote.Option{}, c.options...)` + ctx for that one call.
//
// These tests pin the invariants that hold today:
//   - Repeated WithKeychain / WithAuth / WithUserAgent in the builder
//     chain do not stack duplicate upstream options - the last call wins
//     because they all overwrite an Options field.
//   - WithSegment clones the client without growing options.
//   - Per-request methods do not mutate c.options across many calls.
//   - WithAuth wins over WithKeychain when both are set.
//
// If anyone later adds a runtime *Client.WithKeychain(...) that appends
// directly to c.options without checking for an existing keychain option,
// these tests are the canary.
//
// Ported in spirit from deckhouse-cli's options_test.go
// (TestRemoteWithContext_FinalizesLazily, TestWithKeychain_LastWriteReplaces,
// TestWithPlatform_RepeatedCallsDoNotStack) with the per-architecture
// difference: deckhouse-cli finalises lazily at fetch-time; we finalise
// once at construction.

package client

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/assert"
)

// stubKeychain is a sentinel; we only need a unique type to distinguish
// "the keychain is wired in" from "the default keychain is wired in" via
// pointer identity inside Options.
type stubKeychain struct{ tag string }

func (stubKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

// TestOptions_New_BakedDeterministically verifies that two New() calls with
// the same Option chain produce clients with identical options counts.
// Drift between two equivalent constructions would mean buildRemoteOptions
// has hidden state.
func TestOptions_New_BakedDeterministically(t *testing.T) {
	mk := func() *Client {
		return New("registry.example.com",
			WithInsecure(true),
			WithKeychain(stubKeychain{tag: "k"}),
			WithUserAgent("test/1.0"),
		)
	}

	c1, c2 := mk(), mk()
	assert.Equal(t, len(c1.options), len(c2.options),
		"two builds with identical options must produce identical options slices")
}

// TestOptions_WithKeychain_LastWriteReplaces asserts the central property:
// chaining `WithKeychain(k1), WithKeychain(k2)` in the option list does not
// stack two `remote.WithAuthFromKeychain` entries on c.options. The second
// call must overwrite Options.Keychain, and buildRemoteOptions must emit
// exactly one auth-related option.
//
// Equivalent in spirit to deckhouse-cli's TestWithKeychain_LastWriteReplaces;
// the wire-up differs (we materialise once at construction), the invariant
// is the same.
func TestOptions_WithKeychain_LastWriteReplaces(t *testing.T) {
	baseline := New("registry.example.com", WithInsecure(true))
	withTwoKeychains := New("registry.example.com",
		WithInsecure(true),
		WithKeychain(stubKeychain{tag: "first"}),
		WithKeychain(stubKeychain{tag: "second"}),
	)

	delta := len(withTwoKeychains.options) - len(baseline.options)
	assert.Equal(t, 1, delta,
		"WithKeychain x 2 must add exactly one upstream auth option, not two")
}

// TestOptions_WithAuth_LastWriteReplaces is the symmetrical claim for
// WithAuth: repeated calls overwrite, exactly one remote.WithAuth lands in
// c.options.
func TestOptions_WithAuth_LastWriteReplaces(t *testing.T) {
	baseline := New("registry.example.com", WithInsecure(true))
	withTwoAuths := New("registry.example.com",
		WithInsecure(true),
		WithAuth(authn.Anonymous),
		WithAuth(authn.Anonymous),
	)

	delta := len(withTwoAuths.options) - len(baseline.options)
	assert.Equal(t, 1, delta,
		"WithAuth x 2 must add exactly one upstream auth option, not two")
}

// TestOptions_WithUserAgent_LastWriteReplaces is the third axis: sanity-
// check that a non-auth option with the same overwrite semantics behaves
// the same way.
func TestOptions_WithUserAgent_LastWriteReplaces(t *testing.T) {
	baseline := New("registry.example.com", WithInsecure(true))
	withTwoUAs := New("registry.example.com",
		WithInsecure(true),
		WithUserAgent("first/1.0"),
		WithUserAgent("second/2.0"),
	)

	delta := len(withTwoUAs.options) - len(baseline.options)
	assert.Equal(t, 1, delta,
		"WithUserAgent x 2 must add exactly one upstream UA option, not two")
}

// TestOptions_WithAuthBeatsWithKeychain asserts the documented contract in
// buildRemoteOptions: when both Auth and Keychain are set, only WithAuth
// is emitted (passing both `remote.WithAuth` and `remote.WithAuthFromKeychain`
// to go-containerregistry is an error).
func TestOptions_WithAuthBeatsWithKeychain(t *testing.T) {
	onlyAuth := New("registry.example.com",
		WithInsecure(true),
		WithAuth(authn.Anonymous),
	)
	both := New("registry.example.com",
		WithInsecure(true),
		WithAuth(authn.Anonymous),
		WithKeychain(stubKeychain{tag: "ignored"}),
	)

	assert.Equal(t, len(onlyAuth.options), len(both.options),
		"Auth wins over Keychain - no double auth option")
}

// TestOptions_WithSegment_DoesNotGrowOptions ensures the segment-cloning
// path keeps the parent's options slice intact. WithSegment in this
// architecture must NOT recompute or re-append options - chained calls are
// a common pattern and would otherwise stack N copies of every option.
func TestOptions_WithSegment_DoesNotGrowOptions(t *testing.T) {
	c := New("registry.example.com",
		WithInsecure(true),
		WithKeychain(stubKeychain{tag: "k"}),
		WithUserAgent("test/1.0"),
	)
	want := len(c.options)

	chained := c.
		WithSegment("a").
		WithSegment("b").
		WithSegment("c", "d").
		WithSegment("e").(*Client)

	assert.Equal(t, want, len(chained.options),
		"WithSegment must share the parent options slice (no per-segment growth)")
}

// TestOptions_RequestPathDoesNotMutateClientOptions is our analog of
// deckhouse-cli's TestRemoteWithContext_FinalizesLazily: their finalize
// happens at fetch time, ours happens at construction, but the underlying
// promise is the same - the client's stored options must not grow with
// usage. Each request method does
//
//	opts := append([]remote.Option{}, c.options...)
//	opts = append(opts, c.withContext(ctx))
//
// which creates a fresh slice. Verify across many calls that c.options
// itself is untouched.
func TestOptions_RequestPathDoesNotMutateClientOptions(t *testing.T) {
	_, c := newTestServer(t)
	pushRandomImage(t, c, "repo", "v1")
	initial := len(c.options)

	for i := 0; i < 10; i++ {
		_, _ = c.WithSegment("repo").ListTags(context.Background())
		_, _ = c.WithSegment("repo").ImageExists(context.Background(), "v1")
		_, _ = c.WithSegment("repo").GetDigest(context.Background(), "v1")
	}

	assert.Equal(t, initial, len(c.options),
		"request methods must not append to c.options across calls")
}

// TestOptions_NoAuthNoKeychain_EmitsNeitherAuthOption locks in the negative
// case: with neither Auth nor Keychain set, c.options must not contain any
// remote.WithAuth / remote.WithAuthFromKeychain - go-containerregistry
// defaults to authn.Anonymous when no auth option is supplied.
func TestOptions_NoAuthNoKeychain_EmitsNeitherAuthOption(t *testing.T) {
	withInsecure := New("registry.example.com", WithInsecure(true))
	withInsecureAndKeychain := New("registry.example.com",
		WithInsecure(true),
		WithKeychain(stubKeychain{tag: "k"}),
	)

	// Exactly one more option in the keychain variant; the only difference
	// is the auth option.
	delta := len(withInsecureAndKeychain.options) - len(withInsecure.options)
	assert.Equal(t, 1, delta,
		"the only delta between 'no auth' and 'keychain only' must be a single auth option")
}
