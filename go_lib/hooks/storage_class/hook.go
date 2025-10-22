/*
Copyright 2021 Flant JSC

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

package storage_class

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/regexpset"
)

type StorageClass interface {
	GetName() string
}

type SimpleStorageClass struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (sc *SimpleStorageClass) GetName() string {
	return sc.Name
}

func storageClasses(input *go_hook.HookInput, pathFunc func(path string) string, storageClassesConfig []StorageClass) error {
	excludes, err := regexpset.NewFromValues(input.Values, pathFunc("storageClass.exclude"))
	if err != nil {
		return fmt.Errorf("storageClass.exclude set creation error: %v", err)
	}

	var storageClassesFiltered []StorageClass
	for _, storageClass := range storageClassesConfig {
		needExclude := excludes.Match(storageClass.GetName())
		if !needExclude {
			storageClassesFiltered = append(storageClassesFiltered, storageClass)
		}
	}

	input.Values.Set(pathFunc("internal.storageClasses"), storageClassesFiltered)

	return nil
}

func RegisterHook(moduleName string, storageClassesConfig []StorageClass) bool {
	valuePath := func(path string) string {
		return fmt.Sprintf("%s.%s", moduleName, path)
	}

	handler := func(_ context.Context, input *go_hook.HookInput) error {
		return storageClasses(input, valuePath, storageClassesConfig)
	}

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	}, handler)
}
