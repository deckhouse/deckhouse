/*
Copyright 2023 Flant JSC

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

package imc

import (
	"github.com/deckhouse/deckhouse/go_lib/dependency/storage"
)

var (
	valuesStorage    storage.ValuesStorage
)

func init() {
	valuesStorage = storage.NewStorage(storage.MemoryValuesStorageDriver)
}

// SaveValue could be used in the modules, to store their internal values for updater
// One module does not have access to the other's module values, so we can do it through this interface
func SaveValue(key string, value interface{}) {
	valuesStorage.Set(key, value)
}

// RemoveValue remove previously stored value
func RemoveValue(key string) {
	valuesStorage.Remove(key)
}

// GetValue returns saved value. !Attention: Please don't use it in hooks, only for tests
func GetValue(key string) (interface{}, bool, error) {
	return valuesStorage.Get(key)
}
