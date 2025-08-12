// Copyright 2022 Flant JSC
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

package ssh

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

func TestSSHHostChecks(t *testing.T) {
	t.Run("User passed incorrect count of hosts", func(t *testing.T) {
		nodes := []string{
			"pref-master-0",
			"pref-master-1",
		}
		cases := []struct {
			title      string
			passedHost []session.Host
			warnMsg    string
		}{
			{
				title:      "User passed zero hosts",
				passedHost: nil,
				warnMsg:    notPassedWarn,
			},

			{
				title:      "User passed less hosts than nodes",
				passedHost: []session.Host{{Host: "127.0.0.1", Name: "master-0"}},
				warnMsg:    notEnthoughtWarn,
			},

			{
				title:      "User passed more hosts than nodes",
				passedHost: []session.Host{{Host: "127.0.0.1", Name: "master-0"}, {Host: "127.0.0.2"}, {Host: "127.0.0.3"}},
				warnMsg:    tooManyWarn,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				t.Run("Does not confirm incorrect host warning", func(t *testing.T) {
					nodesToHosts, err := CheckSSHHosts(c.passedHost, nodes, "all-nodes", func(msg string) bool {
						require.Contains(t, msg, c.warnMsg, "Incorrect warning")
						return false
					})

					require.Error(t, err, "should return error")
					require.Nil(t, nodesToHosts)
				})

				t.Run("Confirm incorrect host warning", func(t *testing.T) {
					nodesToHosts, err := CheckSSHHosts(c.passedHost, nodes, "all-nodes", func(msg string) bool {
						require.Contains(t, msg, c.warnMsg, "Incorrect warning")
						return true
					})

					require.NoError(t, err)
					require.Equal(t, nodesToHosts, map[string]string{
						"pref-master-0": "",
						"pref-master-1": "",
					}, "should return empty host for every node")
				})
			})
		}
	})

	t.Run("User passed correct count of hosts", func(t *testing.T) {
		assertNotShowIncorrectCountWarn := func(t *testing.T, msg string) {
			for _, w := range []string{notPassedWarn, notEnthoughtWarn, tooManyWarn} {
				require.NotContains(t, msg, w, "should not show incorrect count warning")
			}
		}

		passedHosts := []session.Host{{Host: "127.0.0.1", Name: "master-0"}, {Host: "127.0.0.2", Name: "master-1"}, {Host: "127.0.0.3", Name: "master-2"}}

		t.Run("Does not confirm nodes to hosts mapping", func(t *testing.T) {
			nodes := []string{"master-0", "master-1", "master-2"}
			nodesToHosts, err := CheckSSHHosts(passedHosts, nodes, "all-nodes", func(msg string) bool {
				assertNotShowIncorrectCountWarn(t, msg)

				require.Contains(t, msg, checkHostsMsg, "Incorrect message")

				return false
			})

			require.Error(t, err, "should return error")
			require.Nil(t, nodesToHosts)
		})

		t.Run("Confirms nodes to hosts mapping", func(t *testing.T) {
			confirmFunc := func(msg string) bool {
				assertNotShowIncorrectCountWarn(t, msg)

				require.Contains(t, msg, checkHostsMsg, "Incorrect message")

				return true
			}

			t.Run("Nodes passed in incorrect order", func(t *testing.T) {
				nodes := []string{"master-1", "master-2", "master-0"}
				nodesToHosts, err := CheckSSHHosts(passedHosts, nodes, "all-nodes", confirmFunc)

				require.NoError(t, err, "should not return error")
				require.Equal(t, nodesToHosts, map[string]string{
					"master-0": passedHosts[0].Host,
					"master-1": passedHosts[1].Host,
					"master-2": passedHosts[2].Host,
				}, "nodes names should sorted")
			})

			t.Run("Nodes passed in correct order", func(t *testing.T) {
				nodes := []string{"master-0", "master-1", "master-2"}
				nodesToHosts, err := CheckSSHHosts(passedHosts, nodes, "all-nodes", confirmFunc)

				require.NoError(t, err, "should not return error")
				require.Equal(t, nodesToHosts, map[string]string{
					"master-0": passedHosts[0].Host,
					"master-1": passedHosts[1].Host,
					"master-2": passedHosts[2].Host,
				}, "nodes names should sorted")
			})

			t.Run("Reducing a cluster to a single master", func(t *testing.T) {
				nodes := []string{"master-0"}
				nodesToHosts, err := CheckSSHHosts(passedHosts, nodes, "scale-to-single-master", confirmFunc)

				require.NoError(t, err, "should not return error")
				require.Equal(t, nodesToHosts, map[string]string{
					"master-0": passedHosts[0].Host,
				}, "nodes name must be the same")
			})

		})
	})
}
