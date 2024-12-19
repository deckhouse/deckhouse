/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouseversion

import (
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name              extenders.ExtenderName = "DeckhouseVersion"
	RequirementsField string                 = "deckhouse"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	logger         *log.Logger
	versionMatcher *versionmatcher.Matcher
}

// TODO: refactor
func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:         log.Default().With("extender", Name),
			versionMatcher: versionmatcher.New(false),
		}

		// try to set deckhouse version from env
		if val := app.TestVarExtenderDeckhouseVersion; val != "" {
			if parsed, err := semver.NewVersion(val); err == nil {
				instance.logger.Debugf("set deckhouse version to the '%s' from env", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
			instance.logger.Debug("failed to parse deckhouse version from env")
		}

		if val := app.VersionDeckhouse; val != "" {
			if parsed, err := semver.NewVersion(app.VersionDeckhouse); err == nil {
				instance.logger.Debugf("set deckhouse version to '%s'", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
		}

		instance.logger.Warn("failed to parse deckhouse version, the 'v2.0.0' version will be used")
	})

	return instance
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		return fmt.Errorf("add constraint for '%s' module: %w", name, err)
	}
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.versionMatcher.DeleteConstraint(name)
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return true
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	if !e.versionMatcher.Has(name) {
		return nil, nil
	}

	if err := e.versionMatcher.Validate(name); err != nil {
		return ptr.To(false), fmt.Errorf("the '%s' module`s requirements not met: the current deckhouse version is not suitable: %v", name, err)
	}

	e.logger.Debugf("the '%s' module`s requirements met", name)
	return ptr.To(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		if name != "" {
			return name, fmt.Errorf("requirements of the '%s' module not met: the '%s' deckhouse version is not suitable: %v", name, baseVersion, err)
		}
		return "", fmt.Errorf("check modules requirements: the '%s' deckhouse version is invalid: %v", baseVersion, err)
	}

	e.logger.Debugf("modules requirements for '%s' deckhouse version met", baseVersion)
	return "", nil
}

func (e *Extender) ValidateRelease(release, constraint string) error {
	if err := e.versionMatcher.ValidateConstraint(constraint); err != nil {
		return fmt.Errorf("the '%s' module release`s requirements not met: the current deckhouse version is not suitable: %v", release, err)
	}

	e.logger.Debugf("the '%s' module release`s requirements met", release)
	return nil
}
