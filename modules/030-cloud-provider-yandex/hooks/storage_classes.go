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

var storageClassesConfig = []storage_class.StorageClass{
	&storage_class.SimpleStorageClass{
		Name: "network-hdd",
		Type: "network-hdd",
	},
	&storage_class.SimpleStorageClass{
		Name: "network-ssd",
		Type: "network-ssd",
	},
	&storage_class.SimpleStorageClass{
		Name: "network-ssd-nonreplicated",
		Type: "network-ssd-nonreplicated",
	},
	&storage_class.SimpleStorageClass{
		Name: "network-ssd-io-m3",
		Type: "network-ssd-io-m3",
	},
}

var _ = storage_class.RegisterHook("cloudProviderYandex", storageClassesConfig)
