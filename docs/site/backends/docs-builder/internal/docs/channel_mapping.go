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

const channelMappingChannels = "channels"

// moduleName - channelMappingChannels - channelCode
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
	Code    string
	Version string
}

// Module represents a module with its name and associated channels
type Module struct {
	ModuleName string
	Channels   []Channel
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

	var modules []Module

	for moduleName, channels := range cm {
		if channelMap, exists := channels[channelMappingChannels]; exists {
			module := Module{
				ModuleName: moduleName,
				Channels:   []Channel{},
			}

			for channelCode, entity := range channelMap {
				module.Channels = append(module.Channels, Channel{
					Code:    channelCode,
					Version: entity.Version,
				})
			}

			// Sort channels by code
			sort.Slice(module.Channels, func(i, j int) bool {
				if module.Channels[i].Code != module.Channels[j].Code {
					return module.Channels[i].Code < module.Channels[j].Code
				}
				return module.Channels[i].Version < module.Channels[j].Version
			})

			modules = append(modules, module)
		}
	}

	// Sort modules by name
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ModuleName < modules[j].ModuleName
	})

	return modules, nil
}

func (m *channelMappingEditor) getModulesCount() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.baseDir, modulesDir, "channels.yaml")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, return 0 modules count
			return 0, nil
		}
		return -1, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var cm = make(map[string]any)

	err = yaml.NewDecoder(f).Decode(&cm)
	if err != nil && !errors.Is(err, io.EOF) {
		return -1, fmt.Errorf("decode yaml: %w", err)
	}

	return len(cm), nil
}
