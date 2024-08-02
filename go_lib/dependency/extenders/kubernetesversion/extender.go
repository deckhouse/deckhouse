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
	"time"

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
	instance       *Extender
	once           sync.Once
	kubernetesOnce sync.Once
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
	})
	return instance
}

// set initial kubernetes version
func (e *Extender) getKubernetesVersion() {
	kubernetesOnce.Do(func() {
		if val := os.Getenv("TEST_EXTENDER_KUBERNETES_VERSION"); val != "" {
			parsed, err := semver.NewVersion(val)
			if err == nil {
				instance.logger.Debugf("setting kubernetes version to %s from env", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
			instance.logger.Warnf("cannot parse TEST_EXTENDER_KUBERNETES_VERSION env variable value %q: %v", val, err)
		}
		if err := e.waitForFileExists("/tmp/kubectl_version"); err != nil {
			e.err = err
			return
		}
		content, err := os.ReadFile("/tmp/kubectl_version")
		if err != nil {
			e.err = err
			return
		}
		parsed, err := semver.NewVersion(strings.TrimSpace(string(content)))
		if err != nil {
			e.err = err
			return
		}
		instance.logger.Debugf("setting kubernets version to %s from file", parsed.String())
		e.versionMatcher.ChangeBaseVersion(parsed)
		go instance.watchForKubernetesVersion()
	})
}

func (e *Extender) waitForFileExists(path string) error {
	e.logger.Debugf("waiting for file %s", path)
	for {
		if _, err := os.Stat(path); err == nil {
			e.logger.Debugf("file %s exists", path)
			return nil
		} else if os.IsNotExist(err) {
			time.Sleep(10 * time.Millisecond)
		} else {
			return err
		}
	}
}

// update kubernetes version if kubectl_version is updated
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
		e.logger.Debugf("adding installed constraint for %q failed", name)
		return err
	}
	e.logger.Debugf("installed constraint for %q is added", name)
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debugf("deleting installed constrain for %q", name)
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
	e.getKubernetesVersion()
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse kubernetes version failed: %s", e.err)}
	}
	e.mtx.Unlock()
	if !e.versionMatcher.Has(name) {
		return nil, nil
	}
	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
		return name, fmt.Errorf("requirements of %s are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
	}
	e.logger.Debugf("requirements for %s are satisfied", baseVersion)
	return "", nil
}

func (e *Extender) ValidateRelease(releaseName, rawConstraint string) error {
	e.getKubernetesVersion()
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return fmt.Errorf("parse kubernetes version failed: %s", e.err)
	}
	e.mtx.Unlock()
	e.logger.Debugf("validate requirements for %s", releaseName)
	if err := e.versionMatcher.Validate(rawConstraint); err != nil {
		e.logger.Errorf("requirements of %s release are not satisfied: current kubernetes version is not suitable: %s", releaseName, err.Error())
		return fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s release are satisfied", releaseName)
	return nil
}
