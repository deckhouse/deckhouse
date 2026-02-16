// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package objectprefix

import (
	"strings"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
)

const (
	ObjectPrefix = "d8-"
)

// NormalizeManagedServicesPrefix walks operations and ensures object names
// have the Deckhouse object prefix (d8-) when in a managed namespace.
func NormalizeManagedServicesPrefix(operations []sdkpkg.PatchCollectorOperation) {
	for _, op := range operations {
		ns := op.GetNamespace()
		name := op.GetName()
		if !needsManagedPrefix(ns) {
			continue
		}
		if strings.HasPrefix(name, ObjectPrefix) {
			continue
		}
		op.SetNamePrefix(ObjectPrefix)
	}
}

// needsManagedPrefix returns true if the namespace is one where object names
func needsManagedPrefix(namespace string) bool {
	return strings.HasPrefix(namespace, "d8-") || strings.HasPrefix(namespace, "d8ms-")
}
