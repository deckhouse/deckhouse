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

package options

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestNewVaultOptions(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		opts := NewVaultOptions()
		assert.Nil(t, opts.Logger)
		assert.Nil(t, opts.Registry)
	})

	t.Run("with logger", func(t *testing.T) {
		logger := log.NewNop()
		opts := NewVaultOptions(WithLogger(logger))
		assert.Equal(t, logger, opts.Logger)
		assert.Nil(t, opts.Registry)
	})

	t.Run("with existing registry", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		opts := NewVaultOptions(WithRegistry(reg))
		assert.Nil(t, opts.Logger)
		assert.Equal(t, reg, opts.Registry)
	})

	t.Run("with new registry", func(t *testing.T) {
		opts := NewVaultOptions(WithNewRegistry())
		assert.Nil(t, opts.Logger)
		assert.NotNil(t, opts.Registry)
	})

	t.Run("multiple options applied in order", func(t *testing.T) {
		logger := log.NewNop()
		reg := prometheus.NewRegistry()
		opts := NewVaultOptions(WithLogger(logger), WithRegistry(reg))
		assert.Equal(t, logger, opts.Logger)
		assert.Equal(t, reg, opts.Registry)
	})

	t.Run("last option wins for same field", func(t *testing.T) {
		reg1 := prometheus.NewRegistry()
		reg2 := prometheus.NewRegistry()
		opts := NewVaultOptions(WithRegistry(reg1), WithRegistry(reg2))
		assert.Equal(t, reg2, opts.Registry)
	})

	t.Run("WithNewRegistry then WithRegistry", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		opts := NewVaultOptions(WithNewRegistry(), WithRegistry(reg))
		assert.Equal(t, reg, opts.Registry)
	})
}

func TestNewRegisterOptions(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		opts := NewRegisterOptions()
		assert.Equal(t, "", opts.Help)
		assert.Nil(t, opts.ConstantLabels)
	})

	t.Run("with help", func(t *testing.T) {
		opts := NewRegisterOptions(WithHelp("help text"))
		assert.Equal(t, "help text", opts.Help)
		assert.Nil(t, opts.ConstantLabels)
	})

	t.Run("with constant labels", func(t *testing.T) {
		labels := map[string]string{"service": "api", "version": "v1"}
		opts := NewRegisterOptions(WithConstantLabels(labels))
		assert.Equal(t, "", opts.Help)
		assert.Equal(t, labels, opts.ConstantLabels)
	})

	t.Run("multiple options", func(t *testing.T) {
		labels := map[string]string{"env": "prod"}
		opts := NewRegisterOptions(WithHelp("metric help"), WithConstantLabels(labels))
		assert.Equal(t, "metric help", opts.Help)
		assert.Equal(t, labels, opts.ConstantLabels)
	})

	t.Run("last help wins", func(t *testing.T) {
		opts := NewRegisterOptions(WithHelp("first"), WithHelp("second"))
		assert.Equal(t, "second", opts.Help)
	})

	t.Run("empty help string", func(t *testing.T) {
		opts := NewRegisterOptions(WithHelp(""))
		assert.Equal(t, "", opts.Help)
	})

	t.Run("nil constant labels", func(t *testing.T) {
		opts := NewRegisterOptions(WithConstantLabels(nil))
		assert.Nil(t, opts.ConstantLabels)
	})

	t.Run("empty constant labels", func(t *testing.T) {
		opts := NewRegisterOptions(WithConstantLabels(map[string]string{}))
		assert.Equal(t, map[string]string{}, opts.ConstantLabels)
	})
}
