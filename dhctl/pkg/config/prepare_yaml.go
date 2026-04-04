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

package config

import "bytes"

func PrepareProviderConfigYAML(index SchemaIndex, cfg []byte) []byte {
	if index.Kind != "DVPClusterConfiguration" {
		return cfg
	}

	// cloud provider dvp has problem.
	// when we add ssh key with multiline like
	// sshPublicKey: |
	//   ssh-rsa AAAAA
	// we can have situation with attach in commander
	// because we save yaml after unmarshal document in cluster
	// we get sshPublicKey in end of document with new line
	// but in commander we pass document and trim
	// after it we got ssh public key without new line
	// and terraform get destructive change
	// for prevent it, we add comment to end of document
	// for prevent trim new line in ssh key
	buf := bytes.NewBuffer(cfg)
	buf.WriteString("\n# comment for safe trim")
	return buf.Bytes()
}
