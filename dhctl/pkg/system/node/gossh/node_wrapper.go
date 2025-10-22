// Copyright 2025 Flant JSC
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

package gossh

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type NodeInterfaceWrapper struct {
	sshClient *Client
}

func NewNodeInterfaceWrapper(sshClient *Client) *NodeInterfaceWrapper {
	if sshClient == nil {
		return nil
	}

	return &NodeInterfaceWrapper{
		sshClient: sshClient,
	}
}

func (n *NodeInterfaceWrapper) Command(name string, args ...string) node.Command {
	log.DebugLn("Starting NodeInterfaceWrapper.command")
	defer log.DebugLn("Stop NodeInterfaceWrapper.command")
	return NewSSHCommand(n.sshClient, name, args...)
}

func (n *NodeInterfaceWrapper) File() node.File {
	return NewSSHFile(n.sshClient.sshClient)
}

func (n *NodeInterfaceWrapper) UploadScript(scriptPath string, args ...string) node.Script {
	log.DebugLn("Starting NodeInterfaceWrapper.UploadScript")
	defer log.DebugLn("Stop NodeInterfaceWrapper.UploadScript")
	return NewSSHUploadScript(n.sshClient, scriptPath, args...)
}

func (n *NodeInterfaceWrapper) Client() *Client {
	return n.sshClient
}
