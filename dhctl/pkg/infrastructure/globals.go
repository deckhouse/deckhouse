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

package infrastructure

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// TODO(nabokikhms): fix package level setters in the following PRs.
//
// Set once at startup via SetGlobals from the resolved *options.Options.
var (
	useTfCache   string
	debugEnabled bool
	tmpDir       string
)

// SetGlobals wires in cache/global options at startup.
// TODO(nabokikhms): fix package level setters in the following PRs.
func SetGlobals(opts *options.Options) {
	if opts == nil {
		return
	}
	useTfCache = opts.Cache.UseTfCache
	debugEnabled = opts.Global.IsDebug
	tmpDir = opts.Global.TmpDir
}

// Re-export the canonical use-cache values so consumers reference them via this
// package (matches what the deleted dhctl/pkg/app constants exposed).
const (
	UseStateCacheAsk = options.UseStateCacheAsk
	UseStateCacheYes = options.UseStateCacheYes
	UseStateCacheNo  = options.UseStateCacheNo
)
