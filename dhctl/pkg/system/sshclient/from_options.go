// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshclient

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// ConfigFromOptions translates a resolved *options.Options into the
// sshclient.Config the SSH cascade consumes. CLI and RPC entry points should
// call this once at startup and thread the resulting Config down — that
// keeps low-level packages (gossh/clissh/frontend/cmd) free of any
// dhctl/pkg/app/options dependency.
func ConfigFromOptions(opts *options.Options) Config {
	if opts == nil {
		return Config{}
	}
	return Config{
		LegacyMode:  opts.SSH.LegacyMode,
		ModernMode:  opts.SSH.ModernMode,
		PrivateKeys: opts.SSH.PrivateKeys,
		Passphrases: opts.SSH.PrivateKeysToPassPhrasesFromConfig,

		Hosts:       opts.SSH.Hosts,
		User:        opts.SSH.User,
		Port:        opts.SSH.Port,
		BastionHost: opts.SSH.BastionHost,
		BastionPort: opts.SSH.BastionPort,
		BastionUser: opts.SSH.BastionUser,
		BastionPass: opts.SSH.BastionPass,
		ExtraArgs:   opts.SSH.ExtraArgs,

		BecomePass: opts.Become.BecomePass,
		TmpDir:     opts.Global.TmpDir,
		IsDebug:    opts.Global.IsDebug,
	}
}
