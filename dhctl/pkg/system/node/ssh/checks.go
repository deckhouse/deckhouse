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
	"fmt"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

const (
	// skip  G101: Potential hardcoded credentials
	//nolint:gosec
	notPassedWarn    = "SSH-hosts was not passed. Maybe you run converge in pod?"
	notEnthoughtWarn = "Not enough master SSH-hosts."
	tooManyWarn      = "Too many master SSH-hosts. Maybe you want to delete nodes, but pass hosts for delete via --ssh-host?"

	checkHostsMsg = "Please check, is correct mapping node name to host?"
	checkWarnMsg  = `Warning! %s
If you lose connection to node, converge may not be finished.
Also, SSH connectivity to another nodes will not check before converge node.

And be attentive when you create new control-plane nodes and change another control-plane instances both.
dhctl can not add new master IP's for connection.

%s
Do you want to continue?
`
)

func CheckSSHHosts(userPassedHosts []session.Host, nodesNames []string, phase string, runConfirm func(string) bool) (map[string]string, error) {
	userPassedHostsLen := len(userPassedHosts)
	replicas := len(nodesNames)

	nodeToHost := make(map[string]string)
	for _, nodeName := range nodesNames {
		nodeToHost[nodeName] = ""
	}

	warnMsg := ""

	switch {
	case userPassedHostsLen == 0:
		warnMsg = notPassedWarn
	case userPassedHostsLen < replicas:
		warnMsg = notEnthoughtWarn
	// Happens only when we make destructive changes to the only master in the cluster and
	// to avoid reporting a warning that the number of replicas does not match the number
	// of servers accessed by ssh when the number of masters is reduced.
	// 1 -> 3 -> update(0) -> (message) -> 1
	case userPassedHostsLen == 3 && replicas == 1 && phase == "scale-to-single-master":
		warnMsg = ""
	case userPassedHostsLen > replicas:
		warnMsg = tooManyWarn
	}

	var nodesSorted []string
	nodesSorted = append(nodesSorted, nodesNames...)
	sort.Strings(nodesSorted)

	forConfirmation := make([]string, userPassedHostsLen)

	for i, host := range userPassedHosts {
		nodeNameTrue := false
		for _, nodeName := range nodesSorted {
			if nodeName == host.Name {
				forConfirmation[i] = fmt.Sprintf("%s -> %s", nodeName, host.Host)
				nodeToHost[nodeName] = host.Host
				nodeNameTrue = true
				break
			}
		}
		if !nodeNameTrue {
			forConfirmation[i] = fmt.Sprintf("%s -> %s (ignored)", host.Name, host.Host)
		}
	}

	if warnMsg != "" {
		msg := fmt.Sprintf(checkWarnMsg, warnMsg, strings.Join(forConfirmation, "\n"))
		if !runConfirm(msg) {
			return nil, fmt.Errorf("Hosts warning was not confirmed.")
		}
	} else {
		msg := fmt.Sprintf("%s\n%s\n", checkHostsMsg, strings.Join(forConfirmation, "\n"))
		if !runConfirm(msg) {
			return nil, fmt.Errorf("Node name to host mapping was not confirmed. Please pass hosts in order.")
		}
	}
	return nodeToHost, nil
}
