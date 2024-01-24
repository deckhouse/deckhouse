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
	"registry-modules-watcher/internal/backends"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetState(t *testing.T) {
	expected := []backends.Version{
		{
			Registry:        "TestReg",
			Module:          "TestModule",
			Version:         "1.0.0",
			ReleaseChannels: []string{"alpha"},
			TarFile:         []byte("test"),
		},
	}
	cache := New()
	cache.SetTar("TestReg", "TestModule", "1.0.0", "alpha", []byte("test"))

	state := cache.GetState()
	assert.Equal(t, expected, state, "GetState return wrong state. Expected %v, got %v", expected, state)
}

func TestSetTar(t *testing.T) {
	cache := New()
	cache.SetTar("TestReg", "TestModule", "1.0.0", "stable", []byte(""))
	cache.SetTar("TestReg", "TestModule", "1.0.0", "beta", []byte(""))
	cache.SetTar("TestReg", "TestModule", "1.0.0", "alpha", []byte(""))
	cache.ResetRange()

	cache.SetTar("TestReg", "TestModule", "1.0.1", "alpha", []byte(""))
	rng := cache.GetState()
	// remove "alpha" tag from 1.0.0 and add to 1.0.1
	assert.Equal(t, 2, len(rng), "Unexpected version range. Expected %v, got %v", 2, len(rng))

	cache.SetTar("TestReg", "TestModule", "1.0.1", "beta", []byte(""))
	cache.SetTar("TestReg", "TestModule", "1.0.2", "alpha", []byte(""))

	rng = cache.GetRange()
	// "stable" tag in 1.0.0, "beta" in 1.0.1 and "alpha" in 1.0.2
	assert.Equal(t, 3, len(rng), "Unexpected version range. Expected %v, got %v", 3, len(rng))
}
