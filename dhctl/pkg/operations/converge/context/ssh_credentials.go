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
	"fmt"
	"slices"
	"strings"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// sshCredentials is how dhctl introduces itself to a set of hosts. One session
// carries one user, so hosts reachable under different users never share a list.
type sshCredentials struct {
	User       string
	BecomePass string
}

// SessionForNode points a copy of the connection settings at one master, under the user
// that master answers to. For the control-plane hook and the ssh readiness checker, which
// reach a single node outside the switcher's own session.
func SessionForNode(c *Context, base *session.Session, host session.Host) (*session.Session, error) {
	creds, err := credentialsForNodes(c, []string{host.Name})
	if err != nil {
		return nil, err
	}

	return switchSession(base, creds, []session.Host{host}), nil
}

// hostsOfOneGeneration keeps hosts a single user reaches: the operator's generation while
// it still has one here, otherwise the masters this converge built. Call it on the hosts
// that survived the IP lookup, or a dropped host decides the generation.
func (s *KubeClientSwitcher) hostsOfOneGeneration(hosts []session.Host) ([]session.Host, error) {
	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return nil, fmt.Errorf("get converge state: %w", err)
	}

	thisConverge := make([]session.Host, 0, len(hosts))
	older := make([]session.Host, 0, len(hosts))

	for _, host := range hosts {
		if slices.Contains(convergeState.ConvergeUserNodes, host.Name) {
			thisConverge = append(thisConverge, host)
			continue
		}

		older = append(older, host)
	}

	if len(older) > 0 {
		return older, nil
	}

	return thisConverge, nil
}

// credentialsForNodes says who dhctl logs in as on the given nodes. The masters this
// converge built answer to the converge user, the ones that predate it to the user dhctl
// started with. Keys stay the caller's: the same keys open both, only the account differs.
func credentialsForNodes(c *Context, nodes []string) (sshCredentials, error) {
	if len(nodes) == 0 {
		return sshCredentials{}, fmt.Errorf("pick ssh credentials: no nodes given")
	}

	convergeState, err := c.ConvergeState()
	if err != nil {
		return sshCredentials{}, fmt.Errorf("get converge state: %w", err)
	}

	var thisConverge, older []string

	for _, node := range nodes {
		if slices.Contains(convergeState.ConvergeUserNodes, node) {
			thisConverge = append(thisConverge, node)
			continue
		}

		older = append(older, node)
	}

	if len(thisConverge) > 0 && len(older) > 0 {
		return sshCredentials{}, fmt.Errorf(
			"pick ssh credentials: %s answer to %s and %s to the user dhctl started with, and one session carries one user",
			strings.Join(thisConverge, ", "), global.ConvergeUserName, strings.Join(older, ", "),
		)
	}

	if len(older) > 0 {
		return operatorCredentials(c)
	}

	// No password at all: the converge user's sudo is NOPASSWD.
	return sshCredentials{User: global.ConvergeUserName}, nil
}

// operatorCredentials are the ones dhctl was started with. They come from the connection
// configuration rather than the live client, which converge moves to another user as it
// recreates masters.
func operatorCredentials(c *Context) (sshCredentials, error) {
	connection := c.SSHProviderInitializer.GetConfig()
	if connection == nil || connection.Config == nil {
		return sshCredentials{}, fmt.Errorf("read the ssh user dhctl started with: no connection configuration")
	}

	if connection.Config.User == "" {
		return sshCredentials{}, fmt.Errorf("read the ssh user dhctl started with: it is empty")
	}

	return sshCredentials{User: connection.Config.User, BecomePass: connection.Config.SudoPassword}, nil
}

// switchSession is the session the client moves to: the hosts and the user change,
// everything the operator configured about how to reach them does not.
func switchSession(settings *session.Session, creds sshCredentials, hosts []session.Host) *session.Session {
	return session.NewSession(session.Input{
		User:            creds.User,
		Port:            settings.Port,
		BastionHost:     settings.BastionHost,
		BastionPort:     settings.BastionPort,
		BastionUser:     settings.BastionUser,
		BastionPassword: settings.BastionPassword,
		ExtraArgs:       settings.ExtraArgs,
		AvailableHosts:  hosts,
		BecomePass:      creds.BecomePass,
	})
}

func hostNames(hosts []session.Host) []string {
	names := make([]string, 0, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Name)
	}

	return names
}

func selectMasterStates(first *NodeState, others []*NodeState) map[string][]byte {
	return selectMasterStatesExcept(first, others, nil)
}

func selectMasterStatesExcept(first *NodeState, others []*NodeState, deleted map[string]struct{}) map[string][]byte {
	states := make(map[string][]byte, len(others)+1)

	for _, st := range append([]*NodeState{first}, others...) {
		if st == nil {
			continue
		}

		if _, deleting := deleted[st.Name]; deleting {
			continue
		}

		states[st.Name] = st.State
	}

	return states
}
