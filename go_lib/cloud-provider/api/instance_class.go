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

// InstanceClassObject is a provider InstanceClass resource usable by common validation rules.
//
// Implementations are instantiated with pointer types, so every provider-defined method
// must be nil-safe. GetName is promoted from metav1.ObjectMeta and is not nil-safe:
// callers must check for an absent object before using it.
type InstanceClassObject interface {
	// GetName returns the resource name.
	GetName() string
	// GroupVersionKind returns the GroupVersionKind for the resource.
	GroupVersionKind() GroupVersionKind
	// GetEtcdDisk returns the etcd disk value for error reporting, or nil when the
	// class defines no dedicated etcd disk. Providers may return any printable
	// representation (the raw field, a map, etc.); the value is shown to the operator
	// in admission errors. Absence is expressed as nil, so callers can test for a
	// dedicated disk with GetEtcdDisk() != nil.
	GetEtcdDisk() any
	// GetNodeGroupConsumers returns names of NodeGroups that use the class.
	GetNodeGroupConsumers() []string
}

// BuildInstanceClassName returns the InstanceClass name generated for a NodeGroup.
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
