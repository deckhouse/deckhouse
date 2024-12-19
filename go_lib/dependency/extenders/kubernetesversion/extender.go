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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name              extenders.ExtenderName = "KubernetesVersion"
	RequirementsField string                 = "kubernetes"

	kubernetesVersionFile = "/tmp/kubectl_version"
)

var (
	instance       *Extender
	once           sync.Once
	kubernetesOnce sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	logger         *log.Logger
	versionMatcher *versionmatcher.Matcher
	mtx            sync.Mutex
	err            error
}

// TODO: refactor
func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:         log.Default().With("extender", Name),
			versionMatcher: versionmatcher.New(true)}
	})
	return instance
}

// set initial kubernetes version
func (e *Extender) getKubernetesVersion() {
	kubernetesOnce.Do(func() {
		// try to set kubernetes version from env
		if val := app.TestVarExtenderKubernetesVersion; val != "" {
			if parsed, err := semver.NewVersion(val); err == nil {
				instance.logger.Debugf("set kubernetes version to the '%s' from env", parsed.String())
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
			instance.logger.Warn("failed to parse the '%s' kubernetes version from env", app.TestVarExtenderKubernetesVersion)
		}

		content, err := e.waitForFileExists(kubernetesVersionFile)
		if err != nil {
			e.err = fmt.Errorf("wait for the '%s' file exists: %w", kubernetesVersionFile, err)
			return
		}

		parsed, err := semver.NewVersion(strings.TrimSpace(string(content)))
		if err != nil {
			e.err = fmt.Errorf("parse the '%s' kubernetes version: %w", strings.TrimSpace(string(content)), err)
			return
		}

		instance.logger.Debugf("set kubernetes version to the '%s' from the '%s' file", parsed.String(), kubernetesVersionFile)
		e.versionMatcher.ChangeBaseVersion(parsed)

		go instance.watchForKubernetesVersion()
	})
}

func (e *Extender) waitForFileExists(path string) ([]byte, error) {
	e.logger.Debugf("wait for the '%s' file", path)
	for {
		if _, err := os.Stat(path); err == nil {
			e.logger.Debugf("the '%s' file exists", path)
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read the '%s' file: %w", path, err)
			}
			if len(content) == 0 {
				e.logger.Debugf("the '%s' file is empty", path)
				continue
			}
			return content, nil
		} else if os.IsNotExist(err) {
			time.Sleep(10 * time.Millisecond)
		} else {
			return nil, fmt.Errorf("stat the '%s' file: %w", path, err)
		}
	}
}

// update kubernetes version if kubectl_version is updated
func (e *Extender) watchForKubernetesVersion() {
	versionCh := make(chan *semver.Version)
	watcher := &versionWatcher{ch: versionCh, logger: e.logger}
	go func() {
		if err := watcher.watch(kubernetesVersionFile); err != nil {
			e.mtx.Lock()
			e.err = err
			e.mtx.Unlock()
			close(versionCh)
		}
	}()
	for version := range versionCh {
		e.logger.Debugf("set the '%s' new kubernetes version", version.String())
		e.versionMatcher.ChangeBaseVersion(version)
	}
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		return fmt.Errorf("add constraint for the '%s' module: %w", name, err)
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

	e.getKubernetesVersion()
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse kubernetes version: %w", e.err)}
	}
	e.mtx.Unlock()

	if err := e.versionMatcher.Validate(name); err != nil {
		return ptr.To(false), fmt.Errorf("the '%s' module`s requirements not met: the current kubernetes version is not suitable: %v", name, err)
	}

	e.logger.Debugf("the '%s' module`s requirements met", name)
	return ptr.To(true), nil
}

func (e *Extender) ValidateBaseVersion(version string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(version); err != nil {
		if name != "" {
			return name, fmt.Errorf("the '%s' module`s requirements not met: the '%s' kubernetes version is not suitable: %v", name, version, err)
		}
		return "", fmt.Errorf("check requirements: the kubernetes version is invalid: %v", err)
	}

	e.logger.Debugf("modules requirements for the '%s' kubernetes version met", version)
	return "", nil
}

func (e *Extender) ValidateRelease(release, constraint string) error {
	e.getKubernetesVersion()
	e.mtx.Lock()
	if e.err != nil {
		e.mtx.Unlock()
		return fmt.Errorf("parse kubernetes version: %w", e.err)
	}
	e.mtx.Unlock()

	if err := e.versionMatcher.ValidateConstraint(constraint); err != nil {
		return fmt.Errorf("the '%s' module`s requirements not met: the current kubernetes version is not suitable: %v", release, err)
	}

	e.logger.Debugf("the '%s' module release`s requirements met", release)
	return nil
}
