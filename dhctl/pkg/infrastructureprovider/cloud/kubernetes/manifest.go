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

package kubernetes

import (
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func IsManifest(change plan.ChangeOp, kind string, logger log.Logger) bool {
	return strings.EqualFold(extractManifestKind(change.After, logger), kind)
}

func extractManifestKind(state map[string]interface{}, logger log.Logger) string {
	v, ok := state["manifest"]
	if !ok || v == nil {
		logger.LogDebugF("State does not have a manifest. Returns empty kind\n")
		return ""
	}

	mv, ok := v.(map[string]any)
	if !ok {
		logger.LogDebugF("manifest is not a map. Returns empty kind\n")
		return ""
	}
	if kind, ok := mv["kind"].(string); ok {
		logger.LogDebugF("extracted manifest kind: %s\n", kind)
		return kind
	}

	logger.LogDebugF("manifest does not have a kind. Returns empty kind\n")
	return ""
}
