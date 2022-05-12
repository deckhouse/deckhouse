package ssh

import (
	"fmt"
	"sort"
	"strings"
)

const (
	notPassedWarn    = "SSH-hosts was not passed. Maybe you run converge in pod?"
	notEnthoughtWarn = "Not enough master SSH-hosts."
	tooManyWarn      = "Too many master SSH-hosts. Maybe you want to delete nodes, but pass hosts for delete via --ssh-host?"
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
		return nil, fmt.Errorf("Node name to host mapping was not confirmed. Please pass hosts in order.")
	}

	return nodeToHost, nil
}
