// Copyright 2021 Flant JSC
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

package hooks

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// order 3 needs to after deckhouse-config/ensure_crd
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 3},
}, discoverDeckhouseEdition)

const UnknownEdition = "Unknown"

var (
	CommaSeparatedEEEditions      = "EE,FE,CSE"
	CommaSeparatedEditionsInOrder = "CE,BE,SE,SE-plus,EE,FE,CSE"
)

func discoverDeckhouseEdition(_ context.Context, input *go_hook.HookInput) error {
	editionFile := "/deckhouse/edition"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		editionFile = os.Getenv("D8_EDITION_TMP_FILE")
	}

	edition := UnknownEdition
	content, err := os.ReadFile(editionFile)
	if err != nil {
		input.Logger.Warn("cannot get deckhouse edition", log.Err(err))
	} else {
		edition = strings.TrimSuffix(string(content), "\n")
	}

	input.Values.Set("global.deckhouseEdition", edition)
	return nil
}

func extractAllEditionsInOrder(logger go_hook.Logger) []string {
	logger.Info(
		"Got all editions in order from build",
		slog.String("comma_separated", CommaSeparatedEditionsInOrder),
	)

	allEditionsParts := strings.SplitSeq(CommaSeparatedEditionsInOrder, ",")
	allEditions := make([]string, 0, 7)
	for ed := range allEditionsParts {
		ed = strings.TrimSpace(ed)
		allEditions = append(allEditions, ed)
	}

	return allEditions
}

func extractEEEditionsFromBuildFlag(logger go_hook.Logger) map[string]struct{} {
	logger.Info(
		"Got EE editions from build",
		slog.String("comma_separated", CommaSeparatedEEEditions),
	)

	eeEditionsParts := strings.SplitSeq(CommaSeparatedEEEditions, ",")
	eeEditions := make(map[string]struct{})
	for ed := range eeEditionsParts {
		ed = strings.TrimSpace(ed)
		eeEditions[ed] = struct{}{}
	}

	return eeEditions
}
