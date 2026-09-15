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

package context

import (
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
)

// sshCredentials is how dhctl introduces itself to a set of hosts. One session
// carries one user, so hosts reachable under different users never share a list.
type sshCredentials struct {
	User       string
	Keys       []session.AgentPrivateKey
	BecomePass string
}

func selectMasterStates(first *NodeState, others []*NodeState, keep func(name string) bool) map[string][]byte {
	states := make(map[string][]byte, len(others)+1)

	for _, st := range append([]*NodeState{first}, others...) {
		if st == nil {
			continue
		}
		if !keep(st.Name) {
			continue
		}
		states[st.Name] = st.State
	}

	return states
}

func keepAllMasters(string) bool { return true }
