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

package app

import (
	adapp "github.com/flant/addon-operator/pkg/app"
	"github.com/spf13/cobra"
)

// Config is the operator configuration. It is an alias of addon-operator's
// Config, so *app.Config is the exact same type addon-operator expects in
// WithConfig: there is no conversion at the boundary.
type Config = adapp.Config

// DefaultTempDir is the fallback temporary directory for hook data.
const DefaultTempDir = adapp.DefaultTempDir

// NewConfig returns a Config preset with addon-operator's hardcoded defaults.
// Layer environment variables on top with envconfig.Load before binding flags.
func NewConfig() *Config {
	return adapp.NewConfig()
}

// ApplyConfig mirrors cfg into addon-operator's package-level globals so code
// that reads them directly, and the debug sub-commands, see the resolved
// values. Nil-safe and idempotent.
func ApplyConfig(cfg *Config) {
	adapp.ApplyConfig(cfg)
}

// BindFlags registers operator CLI flags on cmd, using the current cfg values
// as defaults so an explicit flag always wins. rootCmd receives the hidden
// debug-options helper command.
func BindFlags(cfg *Config, rootCmd, cmd *cobra.Command) {
	adapp.BindFlags(cfg, rootCmd, cmd)
}

// DefineDebugCommands attaches addon-operator's global and module debug
// sub-commands to rootCmd.
func DefineDebugCommands(rootCmd *cobra.Command) {
	adapp.DefineDebugCommands(rootCmd)
}
