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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class"
)

type StorageClass struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	AdditionalField string `json:"additional_field"`
}

func (sc StorageClass) GetName() string {
	return sc.Name
}

var defaultStorageClasses = []StorageClass{
	{
		Name:            "first-hdd",
		Type:            "first-hdd",
		AdditionalField: "first-field",
	},
	{
		Name:            "second-hdd",
		Type:            "second-hdd",
		AdditionalField: "second-field",
	},
	{
		Name:            "third-ssd",
		Type:            "third-ssd",
		AdditionalField: "third-field",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, handleStorageClasses)

func handleStorageClasses(_ context.Context, input *go_hook.HookInput) error {
	excludedPatterns := make([]string, 0, 1)
	for _, pattern := range input.Values.Get("cloudProviderFake.storageClass.exclude").Array() {
		excludedPatterns = append(excludedPatterns, pattern.String())
	}

	desired, err := storage_class.ExcludeByAnchoredPatterns(defaultStorageClasses, excludedPatterns)
	if err != nil {
		return fmt.Errorf("compile cloudProviderFake.storageClass.exclude: %w", err)
	}

	storage_class.SortByName(desired)

	input.Values.Set("cloudProviderFake.internal.storageClasses", desired)

	return nil
}
