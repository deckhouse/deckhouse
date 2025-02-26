// Copyright 2023 Flant JSC
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

package cache

import (
	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetState(t *testing.T) {
	expected := []backends.DocumentationTask{
		{
			Registry:        "TestReg",
			Module:          "TestModule",
			Version:         "1.0.0",
			ReleaseChannels: []string{"alpha"},
			TarFile:         []byte("test"),
			Task:            backends.TaskCreate,
		},
	}
	cache := New()

	ver := internal.VersionData{
		Registry:       "TestReg",
		ModuleName:     "TestModule",
		ReleaseChannel: "alpha",
		Version:        "1.0.0",
		TarFile:        []byte("test"),
	}
	cache.SetTar(ver)

	state := cache.GetState()
	assert.Equal(t, expected, state, "GetState return wrong state. Expected %v, got %v", expected, state)
}

func TestSetTar(t *testing.T) {
	cache := New()

	ver := internal.VersionData{
		Registry:       "TestReg",
		ModuleName:     "TestModule",
		ReleaseChannel: "stable",
		Version:        "1.0.0",
		TarFile:        []byte("test"),
	}
	cache.SetTar(ver)

	ver.ReleaseChannel = "beta"
	cache.SetTar(ver)

	ver.ReleaseChannel = "alpha"
	cache.SetTar(ver)

	ver.ReleaseChannel = "alpha"
	ver.Version = "1.0.1"
	cache.SetTar(ver)
	rng := cache.GetState()
	// remove "alpha" tag from 1.0.0 and add to 1.0.1
	assert.Equal(t, 2, len(rng), "Unexpected version range. Expected %v, got %v", 2, len(rng))
}
