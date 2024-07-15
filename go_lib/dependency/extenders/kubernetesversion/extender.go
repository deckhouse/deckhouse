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
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
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
	logger         logger.Logger
	versionMatcher *versionmatcher.Matcher
	mtx            sync.Mutex
	err            error
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{logger: log.WithField("extender", Name), versionMatcher: versionmatcher.New(true)}
		go instance.watchForKubernetesVersion()
	})
	return instance
}

func (e *Extender) watchForKubernetesVersion() {
	versionCh := make(chan *semver.Version)
	watcher := &versionWatcher{ch: versionCh}
	go func() {
		if err := watcher.watch("/tmp/kubectl_version"); err != nil {
			e.mtx.Lock()
			e.err = err
			e.mtx.Unlock()
			close(versionCh)
		}
	}()
	for version := range versionCh {
		e.logger.Debugf("new kubernetes version: %s", version.String())
		e.versionMatcher.ChangeBaseVersion(version)
	}
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
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse kubernetes version failed: %s", e.err)}
	}
	e.mtx.Unlock()
	if err := e.versionMatcher.ValidateByName(name); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) error {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
		return fmt.Errorf("requirements of %s are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
	}
	e.logger.Debugf("requirements for %s are satisfied", baseVersion)
	return nil
}

func (e *Extender) ValidateConstraint(name, rawConstraint string) error {
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return fmt.Errorf("parse kubernetes version failed: %s", e.err)
	}
	e.mtx.Unlock()
	if err := e.versionMatcher.ValidateConstraint(rawConstraint); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return nil
}
