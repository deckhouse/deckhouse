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

package hooks

import (
	"path/filepath"
	"testing"

	"github.com/flant/addon-operator/sdk"
)

const moduleQueue = "/modules/node-manager"

// addon-operator attributes a values commit from a concurrent queue to the hook
// running now and queues a second ModuleRun for one event, so every hook that
// writes node-manager values runs in the module queue.
func TestValuesWritersShareModuleQueue(t *testing.T) {
	writers := map[string]bool{
		"discover_standby_ng.go": false,
		"get_crds.go":            false,
		"set_ng_priorities.go":   false,
	}
	for _, hook := range sdk.Registry().Hooks() {
		name := filepath.Base(hook.GetPath())
		if _, ok := writers[name]; !ok {
			continue
		}
		writers[name] = true
		if queue := hook.GetConfig().Queue; queue != moduleQueue {
			t.Errorf("%s runs in queue %q, values writers share %s", name, queue, moduleQueue)
		}
	}
	for name, seen := range writers {
		if !seen {
			t.Errorf("%s is not registered", name)
		}
	}
}
