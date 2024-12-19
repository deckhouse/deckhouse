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
	"os"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

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
	err            error
}

// TODO: refactor
func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:         log.Default().With("extender", Name),
			versionMatcher: versionmatcher.New(false),
		}
		if val := os.Getenv("TEST_EXTENDER_DECKHOUSE_VERSION"); val != "" {
			parsed, err := semver.NewVersion(val)
			if err == nil {
				instance.logger.Debugf("setting deckhouse version to %s from env", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
			instance.logger.Warnf("cannot parse TEST_DECKHOUSE_VERSION env variable value %q: %v", val, err)
		}
		if raw, err := os.ReadFile("/deckhouse/version"); err == nil {
			if strings.TrimSpace(string(raw)) == "dev" {
				instance.logger.Warn("this is dev cluster, v2.0.0 will be used")
				return
			}
			if parsed, err := semver.NewVersion(strings.TrimSpace(string(raw))); err == nil {
				instance.logger.Debugf("setting deckhouse version to %s from file", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
			} else {
				instance.logger.Warn("failed to parse deckhouse version")
				instance.err = err
			}
		} else {
			instance.logger.Warn("failed to read deckhouse version from /deckhouse/version")
			instance.err = err
		}
	})
	return instance
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Debugf("adding installed constraint for the '%s' module failed", name)
		return err
	}
	e.logger.Debugf("installed constraint for the '%s' module is added", name)
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debugf("deleting installed constraint for the '%s' module", name)
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
	if e.err != nil {
		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse deckhouse version failed: %s", e.err)}
	}
	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Debugf("requirements of the '%s' module are not satisfied: current deckhouse version is not suitable: %s", name, err.Error())
		return ptr.To(false), fmt.Errorf("requirements are not satisfied: current deckhouse version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of the '%s' module are satisfied", name)
	return ptr.To(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		if name != "" {
			e.logger.Debugf("requirements of the '%s' module are not satisfied: %s deckhouse version is not suitable: %s", name, baseVersion, err.Error())
			return name, fmt.Errorf("requirements of the '%s' module are not satisfied: %s deckhouse version is not suitable: %s", name, baseVersion, err.Error())
		}
		e.logger.Debugf("modules requirements cannot be checked: deckhouse version is invalid: %s", err.Error())
		return "", fmt.Errorf("modules requirements cannot be checked: deckhouse version is invalid: %s", err.Error())
	}
	e.logger.Debugf("modules requirements for '%s' deckhouse version are satisfied", baseVersion)
	return "", nil
}

func (e *Extender) ValidateRelease(releaseName, rawConstraint string) error {
	if e.err != nil {
		return fmt.Errorf("parse deckhouse version failed: %s", e.err)
	}
	if err := e.versionMatcher.ValidateConstraint(rawConstraint); err != nil {
		e.logger.Debugf("requirements of the '%s' module release are not satisfied: current deckhouse version is not suitable: %s", releaseName, err.Error())
		return fmt.Errorf("requirements are not satisfied: current deckhouse version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of the '%s' module release are satisfied", releaseName)
	return nil
}
