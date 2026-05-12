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

// Package options holds the parsed CLI/runtime configuration for dhctl.
//
// It replaces the package-level mutable globals previously kept in
// dhctl/pkg/app. Each domain has its own struct with field-level defaults;
// flag definitions in dhctl/pkg/app populate these structs instead of writing
// to package vars, so operations can be passed an explicit *Options.
package options

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
)

// DefaultTmpDir returns the default location for dhctl temporary state.
// It is derived from os.TempDir at call time so the value cannot be mutated
// from outside the package.
func DefaultTmpDir() string {
	return filepath.Join(os.TempDir(), "dhctl")
}

// DefaultDeckhouseDir is the directory where the dhctl binary expects to find
// version/edition metadata files when running inside the deckhouse container.
const DefaultDeckhouseDir = "/deckhouse"

// Options aggregates every domain-specific options struct used by dhctl.
type Options struct {
	Global       GlobalOptions
	BuildInfo    BuildInfo
	SSH          SSHOptions
	Become       BecomeOptions
	Kube         KubeOptions
	Cache        CacheOptions
	Bootstrap    BootstrapOptions
	Preflight    PreflightOptions
	Converge     ConvergeOptions
	AutoConverge AutoConvergeOptions
	Server       ServerOptions
	Render       RenderOptions
	ControlPlane ControlPlaneOptions
	Destroy      DestroyOptions
	Registry     RegistryOptions
}

// DirConfig returns the directory configuration consumed by pkg/config and
// pkg/template. It bundles the download directories (from GlobalOptions) with
// the version-file path (from BuildInfo) so callers do not need to reach into
// both sub-structs.
func (o *Options) DirConfig() *directoryconfig.DirectoryConfig {
	return &directoryconfig.DirectoryConfig{
		DownloadDir:      o.Global.DownloadDir,
		DownloadCacheDir: o.Global.DownloadCacheDir,
		VersionFilePath:  o.BuildInfo.VersionFile,
	}
}

// New returns Options with built-in defaults applied.
// It is intentionally side-effect free apart from reading well-known
// environment variables (DHCTL_DEBUG, USER) and the build metadata files
// under DefaultDeckhouseDir.
func New() *Options {
	return &Options{
		Global:       NewGlobalOptions(),
		BuildInfo:    LoadBuildInfo(DefaultDeckhouseDir),
		SSH:          NewSSHOptions(),
		Cache:        NewCacheOptions(),
		Bootstrap:    NewBootstrapOptions(),
		Converge:     NewConvergeOptions(),
		AutoConverge: NewAutoConvergeOptions(),
		Render:       NewRenderOptions(),
	}
}
