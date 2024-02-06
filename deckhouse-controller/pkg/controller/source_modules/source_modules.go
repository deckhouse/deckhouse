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

package source_modules

import (
	"sync"
)

type SourceModules struct {
	lck  sync.RWMutex
	dict map[string]string
}

func InitSourceModules() *SourceModules {
	return &SourceModules{dict: make(map[string]string)}
}

func (sm *SourceModules) GetSource(module string) string {
	sm.lck.RLock()
	defer sm.lck.RUnlock()
	return sm.dict[module]
}

func (sm *SourceModules) SetSource(moduleName, moduleSource string) {
	sm.lck.Lock()
	sm.dict[moduleName] = moduleSource
	sm.lck.Unlock()
}
