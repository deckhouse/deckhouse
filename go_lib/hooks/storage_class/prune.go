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
	storagev1 "k8s.io/api/storage/v1"
)

// NewModifiedPrunePredictor builds a Config.PruneIfFunc that deletes a StorageClass whose parameters
// have drifted from what the module now wants.
func NewModifiedPrunePredictor[T comparable](convert func(storagev1.StorageClass) T) PrunePredictor[T] {
	return func(actual storagev1.StorageClass, desired *T) bool {
		if desired == nil {
			return false
		}

		return convert(actual) != *desired
	}
}
