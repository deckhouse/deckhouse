// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package frontend

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// TODO(nabokikhms): fix package level setters in the following PRs.
//
// Set once at startup via SetGlobals from the resolved *options.Options.
var (
	becomePass   string
	tmpDir       string
	debugEnabled bool
)

// SetGlobals wires in options at startup.
// TODO(nabokikhms): fix package level setters in the following PRs.
func SetGlobals(opts *options.Options) {
	if opts == nil {
		return
	}
	becomePass = opts.Become.BecomePass
	tmpDir = opts.Global.TmpDir
	debugEnabled = opts.Global.IsDebug
}
