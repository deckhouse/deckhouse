/*
Copyright 2026 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	storagev1 "k8s.io/api/storage/v1"
)

// NewStaticCollector collects a fixed set of StorageClasses — the model of the providers whose classes are
// disk types known up front rather than discovered from the cloud.
func NewStaticCollector[T any](classes ...T) Collector[T] {
	return func(_ *go_hook.HookInput, _ []storagev1.StorageClass) ([]T, error) {
		return classes, nil
	}
}

// NewConvertingCollector converts the desired classes from the ones the module has already created.
func NewConvertingCollector[T any](convert func(storagev1.StorageClass) T) Collector[T] {
	return func(_ *go_hook.HookInput, existing []storagev1.StorageClass) ([]T, error) {
		if len(existing) == 0 {
			return nil, nil
		}

		classes := make([]T, 0, len(existing))
		for _, storageClass := range existing {
			classes = append(classes, convert(storageClass))
		}

		return classes, nil
	}
}
