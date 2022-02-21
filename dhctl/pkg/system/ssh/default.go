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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

var ErrNotEnoughMastersSSHHosts = fmt.Errorf("Master ssh hosts fix canceled.")

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

func CheckSSHHosts(userPassedHosts []string, nodesToCheck map[string]string, runConfirm func(string) bool) (bool, error) {
	nodesForOutput := make([]string, 0)
	knownHosts := make(map[string]struct{})

	for nodeName, host := range nodesToCheck {
		s := fmt.Sprintf("%s | %s", nodeName, host)
		nodesForOutput = append(nodesForOutput, s)
		knownHosts[host] = struct{}{}
	}

	msg := ""

	switch {
	case len(userPassedHosts) < len(nodesToCheck):
		msg = "Not enough master ssh hosts."
	case len(userPassedHosts) > len(nodesToCheck):
		msg = "Too many master ssh hosts. Maybe you want to delete nodes, but pass hosts for delete via ssh-host?"
	default:
		var notKnownHosts []string
		for _, host := range userPassedHosts {
			if _, ok := knownHosts[host]; !ok {
				notKnownHosts = append(notKnownHosts, host)
			}
		}

		if len(notKnownHosts) > 0 {
			msg = "Found unknown ssh hosts. Maybe you want to delete nodes, but pass hosts for delete via ssh-host?"
		}
	}

	if msg != "" {
		msg := fmt.Sprintf(`Warning! %s
If you lose connection to master, converge may not be finished.
You passed:
%v

Known master hosts from state:
%v

Do you want set hosts from terraform state?
Choose 'N' if you want to fix hosts in the command line argument
`, msg, userPassedHosts, strings.Join(nodesForOutput, "\n"))

		if runConfirm(msg) {
			return true, nil
		}

		return false, ErrNotEnoughMastersSSHHosts
	}

	return false, nil
}
