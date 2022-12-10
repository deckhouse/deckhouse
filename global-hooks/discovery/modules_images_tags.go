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
	"encoding/json"
	"fmt"
	"os"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, discoveryModulesImagesTags)

func discoveryModulesImagesTags(input *go_hook.HookInput) error {
	tagsFile := "/deckhouse/modules/images_tags.json"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		tagsFile = os.Getenv("D8_TAGS_TMP_FILE")
	}

	tagsContent, err := os.ReadFile(tagsFile)
	if err != nil {
		return fmt.Errorf("cannot read images tags files: %w", err)
	}

	var tagsObj map[string]interface{}
	if err := json.Unmarshal(tagsContent, &tagsObj); err != nil {
		return fmt.Errorf("invalid images tags json: %w", err)
	}

	input.Values.Set("global.modulesImages.tags", tagsObj)

	return nil
}
