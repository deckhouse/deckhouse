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

package kubernetesversion

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
)

const (
	Name              extenders.ExtenderName = "KubernetesVersion"
	RequirementsField string                 = "kubernetes"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	enabled        bool
	logger         logger.Logger
	versionMatcher *versionmatcher.Matcher
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{logger: log.WithField("extender", Name), versionMatcher: versionmatcher.New(true)}
		appliedExtenders := os.Getenv("ADDON_OPERATOR_APPLIED_MODULE_EXTENDERS")
		if appliedExtenders != "" && strings.Contains(appliedExtenders, string(Name)) {
			instance.logger.Debug("extender is enabled")
			instance.enabled = true
			go instance.watchForKubernetesVersion()
		} else {
			instance.logger.Debugf("extender is disabled, applied extenders: %s", appliedExtenders)
		}
	})
	return instance
}

func (e *Extender) watchForKubernetesVersion() {
	versionCh := make(chan *semver.Version)
	watcher := &versionWatcher{ch: versionCh}
	go func() {
		if err := watcher.watch("/tmp/kubectl_version"); err != nil {
			e.logger.Errorf("failed to watch /tmp/kubectl_version: %v", err)
			e.enabled = false
			return
		}
	}()
	for version := range versionCh {
		e.logger.Debugf("new kubernetes version: %s", version.String())
		e.versionMatcher.ChangeBaseVersion(version)
	}
}

func IsEnabled() bool {
	if instance == nil {
		Instance()
	}
	return instance.enabled
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Debugf("adding constraint for %q failed", name)
		return err
	}
	e.logger.Debugf("constraint for %q is added", name)
	return nil
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
	if err := e.versionMatcher.ValidateByName(name); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) ReverseValidate(baseVersion string) error {
	if name, err := e.versionMatcher.ReverseValidate(baseVersion); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
		return fmt.Errorf("requirements are not satisfied: %s kubernetes version is not suitable: %s", baseVersion, err.Error())
	}
	e.logger.Debugf("requirements for %s are satisfied", baseVersion)
	return nil
}

func (e *Extender) ValidateConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.ValidateConstraint(rawConstraint); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return nil
}
