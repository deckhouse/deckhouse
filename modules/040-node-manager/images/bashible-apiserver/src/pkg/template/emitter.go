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

package template

import (
	"sync"
	"time"
)

type changesEmitter struct {
	sync.Mutex
	count int
}

func (e *changesEmitter) emitChanges() {
	e.Lock()
	e.count++
	e.Unlock()
}

func (e *changesEmitter) runBufferedEmitter(channel chan struct{}) {
	for {
		// we need sleep to avoid emitting configuration change on batch updates
		// for example on a start - we add all NodeGroupConfigurations, but need to rerender context and checksums only once
		time.Sleep(500 * time.Millisecond)
		e.Lock()
		if e.count > 0 {
			channel <- struct{}{}
			e.count = 0
		}
		e.Unlock()
	}
}
