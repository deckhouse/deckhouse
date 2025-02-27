// Copyright 2023 Flant JSC
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

package docs

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

func newChannelMappingEditor(baseDir string) *channelMappingEditor {
	return &channelMappingEditor{baseDir: baseDir}
}

type channelMappingEditor struct {
	baseDir string
	mu      sync.Mutex
}

type versionEntity struct {
	Version string `json:"version" yaml:"version"`
}

// moduleName - "channels" - channelCode
type channelMapping map[string]map[string]map[string]versionEntity

func (m *channelMappingEditor) edit(fn func(channelMapping)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.baseDir, modulesDir, "channels.yaml")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var cm = make(channelMapping)

	err = yaml.NewDecoder(f).Decode(&cm)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode yaml: %w", err)
	}

	fn(cm)

	err = f.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncate %q: %w", path, err)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek %q: %w", path, err)
	}

	err = yaml.NewEncoder(f).Encode(cm)
	if err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}

	return nil
}

// Channel represents a single channel with its code and version
type Channel struct {
	Code    string `json:"code" yaml:"code"`
	Version string `json:"version" yaml:"version"`
}

// Module represents a module with its name and associated channels
type Module struct {
	ModuleName string    `json:"moduleName" yaml:"moduleName"`
	Channels   []Channel `json:"channels" yaml:"channels"`
}

// ChannelMappingData represents the structure of channel mapping data
type ChannelMappingData struct {
	// Modules provides a slice-based representation of the mapping
	Modules []Module
}

func (m *channelMappingEditor) get() ([]Module, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.baseDir, modulesDir, "channels.yaml")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var cm = make(channelMapping)

	err = yaml.NewDecoder(f).Decode(&cm)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode yaml: %w", err)
	}

	// Populate the ModuleChannels field
	moduleChannels := make(map[string]map[string]string)
	var modules []Module

	for moduleName, channels := range cm {
		if channelMap, exists := channels["channels"]; exists {
			moduleChannels[moduleName] = make(map[string]string)

			// Create a new Module
			module := Module{
				ModuleName: moduleName,
				Channels:   []Channel{},
			}

			for channelCode, entity := range channelMap {
				moduleChannels[moduleName][channelCode] = entity.Version

				// Add channel to the module
				module.Channels = append(module.Channels, Channel{
					Code:    channelCode,
					Version: entity.Version,
				})
			}

			modules = append(modules, module)
		}
	}

	sort.Slice(modules, func(i, j int) bool {
		sort.Slice(modules[i].Channels, func(i, j int) bool {
			return cmp.Or(
				cmp.Less(modules[i].Channels[i].Code, modules[i].Channels[j].Code),
				cmp.Less(modules[i].Channels[i].Version, modules[i].Channels[j].Version),
			)
		})

		sort.Slice(modules[j].Channels, func(i, j int) bool {
			return cmp.Or(
				cmp.Less(modules[j].Channels[i].Code, modules[j].Channels[j].Code),
				cmp.Less(modules[j].Channels[i].Version, modules[j].Channels[j].Version),
			)
		})

		return cmp.Less(modules[i].ModuleName, modules[j].ModuleName)
	})

	return modules, nil
}
