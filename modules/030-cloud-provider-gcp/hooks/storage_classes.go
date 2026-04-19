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
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
)

type StorageClass struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	ReplicationType string `json:"replicationType"`
}

var storageClassesConfig = []StorageClass{
	{
		Name:            "pd-standard-not-replicated",
		Type:            "pd-standard",
		ReplicationType: "none",
	},
	{
		Name:            "pd-standard-replicated",
		Type:            "pd-standard",
		ReplicationType: "regional-pd",
	},
	{
		Name:            "pd-balanced-not-replicated",
		Type:            "pd-balanced",
		ReplicationType: "none",
	},
	{
		Name:            "pd-balanced-replicated",
		Type:            "pd-balanced",
		ReplicationType: "regional-pd",
	},
	{
		Name:            "pd-ssd-not-replicated",
		Type:            "pd-ssd",
		ReplicationType: "none",
	},
	{
		Name:            "pd-ssd-replicated",
		Type:            "pd-ssd",
		ReplicationType: "regional-pd",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, storageClasses)

func storageClasses(_ context.Context, input *go_hook.HookInput) error {
	var excludeStorageClasses []gjson.Result

	if input.Values.Exists("cloudProviderGcp.storageClass.exclude") {
		excludeStorageClasses = input.Values.Get("cloudProviderGcp.storageClass.exclude").Array()
	}

	var excludeCheck = func(storageClassName string) (bool, error) {
		for _, excludePattern := range excludeStorageClasses {
			var r, err = regexp.Compile(excludePattern.String())
			if err != nil {
				return false, err
			}
			if r.MatchString(storageClassName) {
				return true, nil
			}
		}
		return false, nil
	}

	var storageClassesFiltered []StorageClass
	for _, storageClass := range storageClassesConfig {
		isExclude, err := excludeCheck(storageClass.Name)
		if err != nil {
			return err
		}
		if !isExclude {
			storageClassesFiltered = append(storageClassesFiltered, storageClass)
		}
	}

	input.Values.Set("cloudProviderGcp.internal.storageClasses", storageClassesFiltered)

	return nil
}
