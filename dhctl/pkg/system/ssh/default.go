// Copyright 2021 Flant JSC
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
	"fmt"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func NewClientFromFlags() *Client {
	settings := session.NewSession(session.Input{
		AvailableHosts: app.SSHHosts,
		User:           app.SSHUser,
		Port:           app.SSHPort,
		BastionHost:    app.SSHBastionHost,
		BastionPort:    app.SSHBastionPort,
		BastionUser:    app.SSHBastionUser,
		ExtraArgs:      app.SSHExtraArgs,
	})

	return &Client{
		Settings: settings,
	}
}

func NewClientFromFlagsWithHosts() (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewClientFromFlags(), nil
}

func NewInitClientFromFlagsWithHosts(askPassword bool) (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewInitClientFromFlags(askPassword)
}

func NewInitClientFromFlags(askPassword bool) (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, nil
	}

	var sshClient *Client
	var err error

	sshClient, err = NewClientFromFlags().Start()
	if err != nil {
		return nil, err
	}

	if askPassword {
		err = terminal.AskBecomePassword()
		if err != nil {
			return nil, err
		}
	}

	return sshClient, nil
}

func CheckSSHHosts(userPassedHosts []string, nodesNames []string, runConfirm func(string) bool) (map[string]string, error) {
	userPassedHostsLen := len(userPassedHosts)
	replicas := len(nodesNames)

	nodeToHost := make(map[string]string)
	for _, nodeName := range nodesNames {
		nodeToHost[nodeName] = ""
	}

	warnMsg := ""

	switch {
	case userPassedHostsLen == 0:
		warnMsg = "SSH-hosts was not passed. Maybe you run converge in pod?"
	case userPassedHostsLen < replicas:
		warnMsg = "Not enough master SSH-hosts."
	case userPassedHostsLen > replicas:
		warnMsg = "Too many master SSH-hosts. Maybe you want to delete nodes, but pass hosts for delete via --ssh-host?"
	}

	if warnMsg != "" {
		msg := fmt.Sprintf(`Warning! %s
If you lose connection to node, converge may not be finished.
Also SSH connectivity to another nodes will not check before converge.

Do you want to contimue?
`, warnMsg)

		if !runConfirm(msg) {
			return nil, fmt.Errorf("Hosts warning was not confirmed.")
		}

		return nodeToHost, nil
	}

	var nodesSorted []string
	nodesSorted = append(nodesSorted, nodesNames...)
	sort.Strings(nodesSorted)

	forConfirmation := make([]string, userPassedHostsLen)

	for i, host := range userPassedHosts {
		nodeName := nodesNames[i]
		forConfirmation[i] = fmt.Sprintf("%s -> %s", nodeName, host)
		nodeToHost[nodeName] = host
	}

	msg := fmt.Sprintf("Please check, is correct mapping node name to host?\n%s\n", strings.Join(forConfirmation, "\n"))

	if !runConfirm(msg) {
		return nil, fmt.Errorf("Node name to host mapping was not confirmed.")
	}

	return nodeToHost, nil
}
