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

package v1alpha1

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheAirGapGatesAreVisibleWhenTheyRefuse is about a status an operator reads before cutting a
// cluster off from its upstream.
//
// `safeToDropUpstream` and `allReplicasFull` are gates, and their whole purpose is to say no. With
// `omitempty` on a bool, "no" is not a value — it is an absent key, indistinguishable from a field
// the implementation does not have. That is not a hypothetical reading: on a live cluster the
// status showed neither field, and the reasonable conclusion drawn from it was that the two gates
// from the design were unimplemented, when in fact both were false.
//
// So they are serialized always. A `false` an operator can see is a decision; an absence is a
// question about the implementation.
func TestTheAirGapGatesAreVisibleWhenTheyRefuse(t *testing.T) {
	raw, err := json.Marshal(RegistryStorageStatus{})
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))

	assert.Contains(t, fields, "safeToDropUpstream",
		"the gate for going air-gap must state its refusal, not omit it")
	assert.Equal(t, false, fields["safeToDropUpstream"])

	assert.Contains(t, fields, "allReplicasFull",
		"losing the leader being unsurvivable is a fact to publish, not to infer from an absence")
	assert.Equal(t, false, fields["allReplicasFull"])
}

// TestTheFillProgressIsAbsentOnlyWhenNothingRanYet keeps the other half honest.
//
// Progress is a pointer on purpose: no fill task has run is a different statement from a task that
// copied nothing, and zero of zero would claim the second. What must not happen is the pointer
// being set and its numbers then vanishing, which is the same `omitempty` trap one level down —
// `filled: 0` is exactly what the beginning of a fill looks like.
func TestTheFillProgressIsAbsentOnlyWhenNothingRanYet(t *testing.T) {
	nothingRan, err := json.Marshal(RegistryStorageStatus{})
	require.NoError(t, err)
	assert.NotContains(t, string(nothingRan), "fill")

	started, err := json.Marshal(RegistryStorageStatus{Fill: &FillProgress{Total: 459}})
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(started, &fields))
	progress, ok := fields["fill"].(map[string]any)
	require.True(t, ok, "a fill task that has started must report its progress")

	assert.Contains(t, progress, "filled",
		"nothing copied yet is a number, and the operator watches it move")
	assert.EqualValues(t, 0, progress["filled"])
	assert.EqualValues(t, 459, progress["total"])
}

// TestBasicCredentialsUnderstandBothFormsACredentialArrivesIn is the asymmetry that made a cluster's
// cache work while its fill could not authenticate at all.
//
// Credentials reach this type either as a username and a password, or as the
// base64("username:password") that a dockercfg carries. Two consumers of the SAME upstream
// credential read them differently: the pass-through cache decoded the combined form, the fill read
// only the pair. So nodes pulled images through the cache perfectly well, while every fill went out
// anonymous and came back `401 Unauthorized: Auth failed` on every repository — including the
// platform's own, which is when it stopped looking like a permissions quirk and started looking like
// what it was.
func TestBasicCredentialsUnderstandBothFormsACredentialArrivesIn(t *testing.T) {
	t.Run("the pair", func(t *testing.T) {
		username, password := (&Auth{Username: "license-token", Password: "secret"}).BasicCredentials()
		assert.Equal(t, "license-token", username)
		assert.Equal(t, "secret", password)
	})

	t.Run("the combined form, which is what a dockercfg carries", func(t *testing.T) {
		auth := &Auth{Auth: base64.StdEncoding.EncodeToString([]byte("license-token:secret"))}
		username, password := auth.BasicCredentials()
		assert.Equal(t, "license-token", username, "reading only the pair here is a silent 401")
		assert.Equal(t, "secret", password)
	})

	t.Run("a password containing a colon", func(t *testing.T) {
		auth := &Auth{Auth: base64.StdEncoding.EncodeToString([]byte("user:pass:word"))}
		username, password := auth.BasicCredentials()
		assert.Equal(t, "user", username)
		assert.Equal(t, "pass:word", password, "only the first colon separates the two")
	})

	t.Run("whatever Encoded produces is readable back", func(t *testing.T) {
		original := &Auth{Username: "u", Password: "p"}
		username, password := (&Auth{Auth: original.Encoded()}).BasicCredentials()
		assert.Equal(t, "u", username)
		assert.Equal(t, "p", password)
	})

	t.Run("nothing set, and nonsense set", func(t *testing.T) {
		username, password := (&Auth{}).BasicCredentials()
		assert.Empty(t, username)
		assert.Empty(t, password)

		username, password = (&Auth{Auth: "not base64 at all"}).BasicCredentials()
		assert.Empty(t, username)
		assert.Empty(t, password)

		var absent *Auth
		username, password = absent.BasicCredentials()
		assert.Empty(t, username, "a nil credential is not a panic")
		assert.Empty(t, password)
	})
}
