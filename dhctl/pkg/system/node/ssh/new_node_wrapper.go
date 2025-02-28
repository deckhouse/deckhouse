// Copyright 2024 Flant JSC
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
	"golang.org/x/crypto/ssh"
)

type NodeNewInterfaceWrapper struct {
	sshClient *ssh.Client
}

func NewNewNodeInterfaceWrapper(sshClient *ssh.Client) *NodeNewInterfaceWrapper {
	if sshClient == nil {
		return nil
	}

	return &NodeNewInterfaceWrapper{
		sshClient: sshClient,
	}
}

func (n *NodeNewInterfaceWrapper) Command(name string, args ...string) node.Command {
	log.DebugLn("Starting NodeInterfaceWrapper.command")
	defer log.DebugLn("Stop NodeInterfaceWrapper.command")
	return frontend.NewSSHCommand(n.sshClient, name, args...)
}

func (n *NodeNewInterfaceWrapper) File() node.File {
	return frontend.NewSSHFile(n.sshClient)
}

func (n *NodeNewInterfaceWrapper) UploadScript(scriptPath string, args ...string) node.Script {
	log.DebugLn("Starting NodeInterfaceWrapper.UploadScript")
	defer log.DebugLn("Stop NodeInterfaceWrapper.UploadScript")
	return frontend.NewSSHUploadScript(n.sshClient, scriptPath, args...)
}

func (n *NodeNewInterfaceWrapper) Client() *ssh.Client {
	return n.sshClient
}
