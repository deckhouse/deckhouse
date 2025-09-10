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

package provider

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"strings"
)

func genericVMChecker(vmType string) isVMChecker {
	return func(rc plan.ResourceChange) bool {
		return rc.Type == vmType
	}
}

func dvpProviderVMChecker() isVMChecker {
	return func(rc plan.ResourceChange) bool {
		return isKubernetesManifestVirtualMachine(rc.Change, "VirtualMachine")
	}
}

func isKubernetesManifestVirtualMachine(change plan.ChangeOp, kind string) bool {
	return strings.EqualFold(extractManifestKind(change.After), kind)
}

func extractManifestKind(state map[string]interface{}) string {
	v, ok := state["manifest"]
	if !ok || v == nil {
		return ""
	}
	mv, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	if kind, ok := mv["kind"].(string); ok {
		log.DebugF("extractManifestKind: %s\n", kind)
		return kind
	}
	return ""
}
