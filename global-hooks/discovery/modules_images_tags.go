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
	"path/filepath"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/iancoleman/strcase"
)

var re = regexp.MustCompile(`^([0-9]+)-([a-zA-Z-]+)$`)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, discoveryModulesImagesDigests)

func discoveryModulesImagesDigests(input *go_hook.HookInput) error {
	var externalModulesDir string

	digestsFile := "/deckhouse/modules/images_digests.json"

	if env := os.Getenv("EXTERNAL_MODULES_DIR"); env != "" {
		externalModulesDir = filepath.Join(env, "modules")
	}

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		digestsFile = os.Getenv("D8_DIGESTS_TMP_FILE")
		externalModulesDir = "testdata/modules-images-tags/external-modules"
	}

	digestsObj, err := parseImagesDigestsFile(digestsFile)
	if err != nil {
		return err
	}

	if externalModulesDir == "" {
		input.Values.Set("global.modulesImages.digests", digestsObj)
		return nil
	}

	modulesDigestsObj := readModulesImagesDigests(input, externalModulesDir)
	for k, v := range modulesDigestsObj {
		// under no circumstances do we overwrite existing digests
		if _, found := digestsObj[k]; !found {
			digestsObj[k] = v
		}
	}

	input.Values.Set("global.modulesImages.digests", digestsObj)
	return nil
}

func parseImagesDigestsFile(filePath string) (map[string]interface{}, error) {
	digestsContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read images digests files: %w", err)
	}

	var digestsObj map[string]interface{}
	if err := json.Unmarshal(digestsContent, &digestsObj); err != nil {
		return nil, fmt.Errorf("invalid images digests json: %w", err)
	}

	return digestsObj, nil
}

func readModulesImagesDigests(input *go_hook.HookInput, modulesDir string) map[string]interface{} {
	digestsObj := make(map[string]interface{})

	dirItems, err := os.ReadDir(modulesDir)
	if err != nil {
		input.LogEntry.Warning(err)
		return nil
	}

	for _, dirItem := range dirItems {
		evalPath := filepath.Join(modulesDir, dirItem.Name())
		evalPath, err = filepath.EvalSymlinks(evalPath)
		if err != nil {
			input.LogEntry.Warning(err)
			continue
		}

		fi, err := os.Stat(evalPath)
		if err != nil {
			input.LogEntry.Warning(err)
			continue
		}
		if !fi.Mode().IsDir() {
			continue
		}

		moduleDigestsObj, err := parseImagesDigestsFile(filepath.Join(evalPath, "images_digests.json"))
		if err != nil {
			input.LogEntry.Warning(err)
			continue
		}

		moduleNameLowerCamel := strcase.ToLowerCamel(re.ReplaceAllString(dirItem.Name(), "$2"))
		digestsObj[moduleNameLowerCamel] = moduleDigestsObj
	}
	return digestsObj
}
