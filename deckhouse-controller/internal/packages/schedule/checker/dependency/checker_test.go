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

// Package dependency provides a checker for validating package dependencies.
// It verifies that required dependencies exist, are enabled, and satisfy version constraints.
package dependency

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func Test_Check(t *testing.T) {
	t.Run("get info error", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			return nil, errors.New("test error")
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   true,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "test error", result.Message)
		assert.False(t, result.Enabled)
	})
	t.Run("module not exist. Optional", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			return &ModuleInfo{}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   true,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "", result.Message)
		assert.True(t, result.Enabled)
	})
	t.Run("not enabled. Optional", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			return &ModuleInfo{IsModuleEnabled: ptr.To(false)}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   true,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "", result.Message)
		assert.True(t, result.Enabled)
	})
	t.Run("enabled. !Optional", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			return &ModuleInfo{IsModuleEnabled: ptr.To(false)}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   false,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "dependency 'test' not enabled", result.Message)
		assert.False(t, result.Enabled)
	})
	t.Run("module not exist. !Optional", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			return &ModuleInfo{}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   false,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "dependency 'test' not found", result.Message)
		assert.False(t, result.Enabled)
	})
	t.Run("not valid version", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			v, _ := semver.NewVersion("1.2")
			return &ModuleInfo{IsModuleEnabled: ptr.To(true), Version: v}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   false,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "1.2.0 is less than 1.21", result.Message)
		assert.False(t, result.Enabled)
	})
	t.Run("pass", func(t *testing.T) {
		f := func(moduleName string) (*ModuleInfo, error) {
			v, _ := semver.NewVersion("1.25")
			return &ModuleInfo{IsModuleEnabled: ptr.To(true), Version: v}, nil
		}

		constraint, _ := semver.NewConstraint(">=1.21")
		dep := map[string]Dependency{
			"test": {
				Constraint: constraint,
				Optional:   false,
			},
		}

		checker := NewChecker(f, dep, log.NewLogger())
		result := checker.Check()
		assert.Equal(t, "", result.Message)
		assert.True(t, result.Enabled)
	})
}

func TestRemovePrereleaseAndMetadata(t *testing.T) {
	ver := semver.MustParse("1.21")
	newVer := removePrereleaseAndMetadata(ver, log.NewLogger())
	assert.Equal(t, ver.String(), newVer.String())

	verPre := semver.MustParse("1.75.0-pr17646+5019b18")
	ver = semver.MustParse("1.75.0")
	newVer = removePrereleaseAndMetadata(verPre, log.NewLogger())
	assert.Equal(t, ver.String(), newVer.String())
}
