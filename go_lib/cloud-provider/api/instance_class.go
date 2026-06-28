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

package api

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// InstanceClass is a provider-specific instance class resource.
type InstanceClass struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec,omitempty"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

// InstanceClassSpec holds provider-specific instance class parameters.
type InstanceClassSpec struct {
	EtcdDisk map[string]any `json:"etcdDisk,omitempty"`
}

// InstanceClassStatus holds runtime status fields populated by the provider module.
type InstanceClassStatus struct {
	NodeGroupConsumers []any `json:"nodeGroupConsumers,omitempty"`
}

// BuildInstanceClassName returns the DVPInstanceClass name generated for a NodeGroup.
func BuildInstanceClassName(nodeGroupName string) string {
	const (
		// Kubernetes DNS-1123 labels are limited to 63 characters.
		nameMaxLength = 63
		// 12 hex characters keep a 48-bit SHA-256 prefix: compact enough for DNS names
		// and less collision-prone than an 8-character/32-bit suffix.
		hashLength = 12
		// One character is reserved for the separator between the readable prefix and hash.
		prefixLength = nameMaxLength - hashLength - 1
	)

	hash := sha256.Sum256([]byte(nodeGroupName))
	suffix := fmt.Sprintf("%x", hash)[:hashLength]

	// Keep the beginning of the NodeGroup name readable while reserving enough
	// space for the separator and hash suffix inside the DNS-1123 length limit.
	prefix := nodeGroupName
	if len(prefix) > prefixLength {
		prefix = prefix[:prefixLength]
	}

	// Truncation can leave the readable prefix ending with a dash, which would
	// create a double separator or an invalid DNS label when joined with the hash.
	prefix = strings.TrimRight(prefix, "-")

	return prefix + "-" + suffix
}
