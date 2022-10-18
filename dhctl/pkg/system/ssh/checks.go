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
)

const (
	// skip  G101: Potential hardcoded credentials
	//nolint:gosec
	notPassedWarn    = "SSH-hosts was not passed. Maybe you run converge in pod?"
	notEnthoughtWarn = "Not enough master SSH-hosts."
	tooManyWarn      = "Too many master SSH-hosts. Maybe you want to delete nodes, but pass hosts for delete via --ssh-host?"

	checkHostsMsg = "Please check, is correct mapping node name to host?"
)

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
		warnMsg = notPassedWarn
	case userPassedHostsLen < replicas:
		warnMsg = notEnthoughtWarn
	case userPassedHostsLen > replicas:
		warnMsg = tooManyWarn
	}

	if warnMsg != "" {
		msg := fmt.Sprintf(`Warning! %s
If you lose connection to node, converge may not be finished.
Also, SSH connectivity to another nodes will not check before converge node.

And be attentive when you create new control-plane nodes and change another control-plane instances both.
dhctl can not add new master IP's for connection.

Do you want to continue?
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
		nodeName := nodesSorted[i]
		forConfirmation[i] = fmt.Sprintf("%s -> %s", nodeName, host)
		nodeToHost[nodeName] = host
	}

	msg := fmt.Sprintf("%s\n%s\n", checkHostsMsg, strings.Join(forConfirmation, "\n"))

	if !runConfirm(msg) {
		return nil, fmt.Errorf("Node name to host mapping was not confirmed. Please pass hosts in order.")
	}

	return nodeToHost, nil
}
