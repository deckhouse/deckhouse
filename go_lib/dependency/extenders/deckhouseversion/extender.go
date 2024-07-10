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
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
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
	logger         logger.Logger
	versionMatcher *versionmatcher.Matcher
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:         log.WithField("extender", Name),
			versionMatcher: versionmatcher.New(),
		}
		if raw, err := os.ReadFile("/deckhouse/version"); err == nil {
			if parsed, err := semver.NewVersion(string(raw)); err == nil {
				instance.versionMatcher.SetBaseVersion(parsed)
			}
		} else {
			instance.logger.Warn("failed to read deckhouse version from /deckhouse/version, v0.0.0 will be used")
		}
	})
	return instance
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Debugf("adding constraint for %q failed", name)
		return err
	}
	e.logger.Debugf("constraint for %q is added", name)
	return nil
}

func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current deckhouse version is not suitable: %s", name, err.Error())
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: current deckhouse version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) IsTerminator() bool {
	return true
}
