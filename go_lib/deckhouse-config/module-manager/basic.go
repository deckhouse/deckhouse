/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package module_manager

import (
	"context"
	"os"

	"github.com/flant/addon-operator/pkg/module_manager"
)

// InitBasic creates basic ModuleManager without additional components.
// It is sufficient to list modules and validate values. It is suitable
// for separated webhooks and tests.
func InitBasic(globalHooksDir string, modulesDir string) (*module_manager.ModuleManager, error) {
	tempDir := os.Getenv("ADDON_OPERATOR_TMP_DIR")
	if tempDir == "" {
		tempDir = "."
	}

	dirs := module_manager.DirectoryConfig{
		ModulesDir:     modulesDir,
		GlobalHooksDir: globalHooksDir,
		TempDir:        tempDir,
	}
	cfg := module_manager.ModuleManagerConfig{
		DirectoryConfig: dirs,
	}
	mm := module_manager.NewModuleManager(context.Background(), &cfg)

	err := mm.Init()
	if err != nil {
		return nil, err
	}

	return mm, nil
}
