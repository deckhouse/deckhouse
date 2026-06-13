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
	"context"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

func IsManifest(change plan.ChangeOp, kind string) bool {
	return strings.EqualFold(extractManifestKind(change.After), kind)
}

func extractManifestKind(state map[string]interface{}) string {
	ctx := context.Background()

	v, ok := state["manifest"]
	if !ok || v == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "State does not have a manifest. Returning empty kind")
		return ""
	}

	mv, ok := v.(map[string]any)
	if !ok {
		dhlog.FromContext(ctx).DebugContext(ctx, "manifest is not a map. Returning empty kind")
		return ""
	}
	if kind, ok := mv["kind"].(string); ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("extracted manifest kind: %s", kind))
		return kind
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "manifest does not have a kind. Returning empty kind")
	return ""
}
