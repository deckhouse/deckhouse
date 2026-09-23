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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestThePersistedPKIIsFoundWhereverItStillIs is about not reissuing a certificate authority.
//
// The state moved out of the secret the storage pod mounts, because that mount handed every
// container in the pod the authority's private key and the token signing key. Moving it creates one
// moment where it matters which secret is read: a cluster that was running the previous render has
// the state only in the old place, and the new secret cannot appear until the hook has put the state
// into the values it is rendered from. Read only the new name and that cluster generates a fresh
// authority — every node agent then trusts a certificate the storage no longer presents, and every
// pull in the cluster fails until the layouts have been rewritten and applied.
func TestThePersistedPKIIsFoundWhereverItStillIs(t *testing.T) {
	cases := []struct {
		name  string
		found []pkiStateSnapshot
		want  string
	}{{
		name: "nothing persisted yet, so the hook generates",
		want: "",
	}, {
		name:  "the state secret alone, which is the ordinary case",
		found: []pkiStateSnapshot{{Secret: PKIStateSecretName, State: []byte("new")}},
		want:  "new",
	}, {
		// A cluster upgraded from the render that kept the state in the mounted secret.
		name:  "only the old location still has it",
		found: []pkiStateSnapshot{{Secret: PKISecretName, State: []byte("old")}},
		want:  "old",
	}, {
		// Both exist for exactly one reconciliation, until the mounted secret is re-rendered
		// without the state. The one the hook now writes wins.
		name: "both, during the change",
		found: []pkiStateSnapshot{
			{Secret: PKISecretName, State: []byte("old")},
			{Secret: PKIStateSecretName, State: []byte("new")},
		},
		want: "new",
	}, {
		name:  "a secret that carries no state at all",
		found: []pkiStateSnapshot{{Secret: PKIStateSecretName}},
		want:  "",
	}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, string(preferredPKIState(testCase.found)))
		})
	}
}
