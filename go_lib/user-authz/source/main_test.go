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

package source

import (
	"testing"

	"go.uber.org/goleak"
)

// This is the one package here that starts goroutines: Run spawns the informer, the sync wait and
// the rebuild loop, and every one of them is supposed to end when its context does. The tests
// cancel and return without waiting, so without this check a goroutine that outlived its context
// would simply run on into the next test and nobody would notice - which is the shape of the bug
// this component can least afford, since these goroutines hold watches against the API server.
//
// The ignore is client-go's own: its reflector logging spins up a background flusher on first use
// and never stops it.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("k8s.io/klog/v2.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("k8s.io/klog.(*loggingT).flushDaemon"),
	)
}
