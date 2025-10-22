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
	storage_class.SimpleStorageClass
	AdditionalField string `json:"additional_field"`
}

var storageClassesConfig = []storage_class.StorageClass{
	&SC{
		SimpleStorageClass: storage_class.SimpleStorageClass{
			Name: "first-hdd",
			Type: "first-hdd",
		},

		AdditionalField: "first-field",
	},

	&SC{
		SimpleStorageClass: storage_class.SimpleStorageClass{
			Name: "second-hdd",
			Type: "second-hdd",
		},

		AdditionalField: "second-field",
	},

	&SC{
		SimpleStorageClass: storage_class.SimpleStorageClass{
			Name: "third-ssd",
			Type: "third-ssd",
		},

		AdditionalField: "third-field",
	},
}

var _ = storage_class.RegisterHook("cloudProviderFake", storageClassesConfig)
