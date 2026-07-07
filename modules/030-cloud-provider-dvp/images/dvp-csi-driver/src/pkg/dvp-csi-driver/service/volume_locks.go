/*
Copyright 2025 Flant JSC

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

package service

import "sync"

type volumeLocks struct {
	mu    sync.Mutex
	locks map[string]struct{}
}

func newVolumeLocks() *volumeLocks {
	return &volumeLocks{
		locks: make(map[string]struct{}),
	}
}

func (vl *volumeLocks) tryAcquire(key string) bool {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	if _, held := vl.locks[key]; held {
		return false
	}

	vl.locks[key] = struct{}{}
	return true
}

func (vl *volumeLocks) release(key string) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	delete(vl.locks, key)
}

func volumeLockKey(volumeID, targetPath string) string {
	return volumeID + "\x00" + targetPath
}
