/*
Copyright 2023 The Kubernetes Authors.
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

package iptables

import "regexp"

var (
	kubeletChainsRegex = regexp.MustCompile(`(?m)^:(KUBE-IPTABLES-HINT|KUBE-KUBELET-CANARY)`)
	ruleEntryRegex     = regexp.MustCompile(`(?m)^-`)
)

// hasKubeletChains checks if the output of an iptables*-save command
// contains any of the rules set by kubelet.
func hasKubeletChains(output []byte) bool {
	return kubeletChainsRegex.Match(output)
}

// ruleEntriesNum counts how many rules there are in an iptables*-save command
// output.
func ruleEntriesNum(iptablesOutput []byte) int {
	return len(ruleEntryRegex.FindAllIndex(iptablesOutput, -1))
}
