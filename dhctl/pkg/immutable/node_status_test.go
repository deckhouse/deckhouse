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

package immutable

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The machine reports its progress to whoever configured it and to nobody else,
// and the conditions are what tell a node still working from one that will never
// register.
func TestFetchNodeStatus(t *testing.T) {
	var gotAuthorization, gotPath string
	machine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{
			"phase": "Failed",
			"message": "no interface of this machine is in 10.0.0.0/16",
			"conditions": [{"type": "AddressResolved", "status": "False", "reason": "NoMatch", "message": "eno1 (192.168.0.59)"}]
		}`)
	}))
	t.Cleanup(machine.Close)

	status, err := FetchNodeStatus(t.Context(), strings.TrimPrefix(machine.URL, "http://"), "a-status-token")

	require.NoError(t, err)
	require.Equal(t, "Bearer a-status-token", gotAuthorization)
	require.Equal(t, nodeStatusPath, gotPath)
	require.Equal(t, PhaseFailed, status.Phase)

	resolved := status.Condition(ConditionAddressResolved)
	require.NotNil(t, resolved)
	require.Equal(t, ConditionFalse, resolved.Status)
	require.Equal(t, "NoMatch", resolved.Reason)
	require.Nil(t, status.Condition("SomethingElse"))

	require.Contains(t, DescribeNodeStatus(status), "AddressResolved=False")
	require.Contains(t, DescribeNodeStatus(status), "10.0.0.0/16")
}

// A machine that refuses the token is not a machine reporting anything: the
// caller must not read a refusal as a status.
func TestFetchNodeStatusRefusal(t *testing.T) {
	machine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "unknown token")
	}))
	t.Cleanup(machine.Close)

	_, err := FetchNodeStatus(t.Context(), strings.TrimPrefix(machine.URL, "http://"), "wrong")

	require.ErrorContains(t, err, "401")
	require.ErrorContains(t, err, "unknown token")
}
