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

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	path := filepath.Join(m.baseDir, "data/modules/channels.yaml")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}

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
