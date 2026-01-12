// Copyright 2025 Flant JSC
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

package fsprovider

import (
	"fmt"
	"os"
	"path"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type DIParams struct {
	InfraVersionsFile string
	BinariesDir       string
	CloudProviderDir  string
	PluginsDir        string
}

func isDir(dir string, errPrefix string) error {
	if !path.IsAbs(dir) {
		return fmt.Errorf("%s is not an absolute path", dir)
	}

	if !fs.IsDirExists(dir) {
		return fmt.Errorf("%s dir '%s' is empty or does not exists", errPrefix, dir)
	}

	return nil
}

func isNotRootDir(dir string, errPrefix string) error {
	if path.Clean(dir) == "/" {
		return fmt.Errorf("%s dir '%s' should not be /", errPrefix, dir)
	}

	if err := isDir(dir, errPrefix); err != nil {
		return err
	}

	return nil
}

func isFile(file string, errPrefix string) error {
	if !path.IsAbs(file) {
		return fmt.Errorf("%s is not an absolute path", file)
	}

	stat, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("%s file '%s' does not exist or got another fs error: %w", errPrefix, file, err)
	}

	if stat.IsDir() {
		return fmt.Errorf("%s '%s' is not file", errPrefix, file)
	}

	return nil
}

func GetDi(logger log.Logger, params *DIParams) (*cloud.ProviderDI, error) {
	if params == nil {
		return nil, fmt.Errorf("no fs.DI params provided")
	}

	if err := isDir(params.BinariesDir, "BinariesDir"); err != nil {
		return nil, err
	}

	if err := isFile(params.InfraVersionsFile, "InfraVersionsFile"); err != nil {
		return nil, err
	}

	if err := isNotRootDir(params.CloudProviderDir, "CloudProviderDir"); err != nil {
		return nil, err
	}

	if err := isDir(params.PluginsDir, "PluginsDir"); err != nil {
		return nil, err
	}

	return &cloud.ProviderDI{
		SettingsProvider:    newSettingsProvider(logger, params.InfraVersionsFile, loadOrGetStore),
		InfraUtilProvider:   newInfrastructureUtilProvider(logger, params.BinariesDir),
		InfraPluginProvider: newPluginsProvider(logger, params.PluginsDir),
		ModulesProvider:     newModulesProvider(logger, params.CloudProviderDir),
	}, nil
}
