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

package hooks

import (
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
)

type StorageClass struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	CachingMode       string `json:"cachingMode,omitempty"`
	DiskIOPSReadWrite int64  `json:"diskIOPSReadWrite,omitempty"`
	DiskMBpsReadWrite int64  `json:"diskMBpsReadWrite,omitempty"`
	Tags              string `json:"tags,omitempty"`
}

var storageClassesConfig = []StorageClass{
	{
		Name: "managed-standard-ssd",
		Type: "StandardSSD_LRS",
	},
	{
		Name: "managed-standard",
		Type: "Standard_LRS",
	},
	{
		Name: "managed-premium",
		Type: "Premium_LRS",
	},
	{
		Name:        "managed-standard-ssd-large",
		Type:        "StandardSSD_LRS",
		CachingMode: "None",
	},
	{
		Name:        "managed-standard-large",
		Type:        "Standard_LRS",
		CachingMode: "None",
	},
	{
		Name:        "managed-premium-large",
		Type:        "Premium_LRS",
		CachingMode: "None",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, storageClasses)

func storageClasses(input *go_hook.HookInput) error {
	var provision []gjson.Result
	if input.Values.Exists("cloudProviderAzure.storageClass.provision") {
		provision = input.Values.Get("cloudProviderAzure.storageClass.provision").Array()
	}

	var provisionStorageClasses []StorageClass

	for _, sc := range provision {
		provisionStorageClasses = append(provisionStorageClasses, StorageClass{
			Name:              sc.Get("name").String(),
			Type:              sc.Get("type").String(),
			DiskIOPSReadWrite: sc.Get("diskIOPSReadWrite").Int(),
			DiskMBpsReadWrite: sc.Get("diskMBpsReadWrite").Int(),
			Tags:              sc.Get("tags").String(),
		})
	}

	var excludeStorageClasses []gjson.Result

	if input.Values.Exists("cloudProviderAzure.storageClass.exclude") {
		excludeStorageClasses = input.Values.Get("cloudProviderAzure.storageClass.exclude").Array()
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

	var storageClasses []StorageClass
	for _, storageClass := range storageClassesConfig {
		isExclude, err := excludeCheck(storageClass.Name)
		if err != nil {
			return err
		}
		if !isExclude {
			storageClasses = append(storageClasses, storageClass)
		}
	}

	storageClasses = append(storageClasses, provisionStorageClasses...)

	if storageClasses == nil {
		input.Values.Set("cloudProviderAzure.internal.storageClasses", []StorageClass{})
	} else {
		input.Values.Set("cloudProviderAzure.internal.storageClasses", storageClasses)
	}

	if input.Values.Exists("cloudProviderAzure.storageClass.default") {
		input.Values.Set("cloudProviderAzure.internal.defaultStorageClass", input.Values.Get("cloudProviderAzure.storageClass.default").String())
	} else {
		input.Values.Remove("cloudProviderAzure.internal.defaultStorageClass")
	}

	return nil
}
