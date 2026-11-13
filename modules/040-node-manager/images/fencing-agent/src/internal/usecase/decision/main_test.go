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

package decision

import (
	"os"
	"testing"
	"time"
)

// TestMain puts the probe cycle numbers on the clock of the tests: in a
// synctest bubble time.Now is the fake time.
func TestMain(m *testing.M) {
	bootMillis = func() uint64 { return uint64(time.Now().UnixMilli()) }

	os.Exit(m.Run())
}
