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
	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class"
)

type SC struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	AdditionalField string `json:"additional_field"`
}

var storageClassesConfig = []SC{
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

var _ = storage_class.RegisterHook(storage_class.Config[SC]{
	ModuleName:      "common",
	ModuleValuesKey: "cloudProviderFake",
	Order:           20,

	ExcludeStorageClassesValuesPath: "cloudProviderFake.storageClass.exclude",
	StorageClassesValuesPath:        "cloudProviderFake.internal.storageClasses",

	NameOfFunc:  func(class SC) string { return class.Name },
	CollectFunc: storage_class.NewStaticCollector(storageClassesConfig...),
})
