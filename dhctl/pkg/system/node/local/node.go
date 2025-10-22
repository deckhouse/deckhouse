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

package local

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type NodeInterface struct{}

func NewNodeInterface() *NodeInterface {
	return &NodeInterface{}
}

func (n *NodeInterface) Command(name string, args ...string) node.Command {
	log.DebugLn("Starting NodeInterface.Command")
	defer log.DebugLn("Stop NodeInterface.Command")

	return NewCommand(name, args...)
}

func (n *NodeInterface) File() node.File {
	return NewFile()
}

func (n *NodeInterface) UploadScript(scriptPath string, args ...string) node.Script {
	log.DebugLn("Starting NodeInterface.UploadScript")
	defer log.DebugLn("Stop NodeInterface.UploadScript")
	return NewScript(scriptPath, args...)
}
