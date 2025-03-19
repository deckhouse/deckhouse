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
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "KubernetesVersion"
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
		instance = &Extender{logger: log.Default().With("extender", Name), versionMatcher: versionmatcher.New(true)}
	})
	return instance
}

// set initial kubernetes version
func (e *Extender) getKubernetesVersion() {
	kubernetesOnce.Do(func() {
		if val := os.Getenv("TEST_EXTENDER_KUBERNETES_VERSION"); val != "" {
			parsed, err := semver.NewVersion(val)
			if err == nil {
				instance.logger.Debug("setting kubernets version from env to", slog.String("version", parsed.String()))
				instance.versionMatcher.ChangeBaseVersion(parsed)
				return
			}
			instance.logger.Warn("cannot parse TEST_EXTENDER_KUBERNETES_VERSION env variable value", slog.String("value", val), log.Err(err))
		}
		content, err := e.waitForFileExists("/tmp/kubectl_version")
		if err != nil {
			e.err = err
			return
		}
		parsed, err := semver.NewVersion(strings.TrimSpace(string(content)))
		if err != nil {
			e.err = err
			return
		}
		instance.logger.Debug("setting kubernets version from file to", slog.String("version", parsed.String()))
		e.versionMatcher.ChangeBaseVersion(parsed)
		go instance.watchForKubernetesVersion()
	})
}

func (e *Extender) waitForFileExists(path string) ([]byte, error) {
	e.logger.Debug("waiting for", slog.String("file", path))
	for {
		if _, err := os.Stat(path); err == nil {
			e.logger.Debug("file exists", slog.String("path", path))
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if len(content) == 0 {
				e.logger.Debug("file is empty", slog.String("path", path))
				continue
			}
			return content, nil
		} else if os.IsNotExist(err) {
			time.Sleep(10 * time.Millisecond)
		} else {
			return nil, err
		}
	}
}

// update kubernetes version if kubectl_version is updated
func (e *Extender) watchForKubernetesVersion() {
	versionCh := make(chan *semver.Version)
	watcher := &versionWatcher{ch: versionCh, logger: e.logger}
	go func() {
		if err := watcher.watch("/tmp/kubectl_version"); err != nil {
			e.mtx.Lock()
			e.err = err
			e.mtx.Unlock()
			close(versionCh)
		}
	}()
	for version := range versionCh {
		e.logger.Debug("new kubernetes version", slog.String("version", version.String()))
		e.versionMatcher.ChangeBaseVersion(version)
	}
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Debug("adding installed constraint for the module failed", slog.String("name", name))
		return err
	}
	e.logger.Debug("installed constraint for the module is added", slog.String("name", name))
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debug("deleting installed constrain for the module", slog.String("name", name))
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
		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse kubernetes version failed: %s", e.err)}
	}
	e.mtx.Unlock()
	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Debug("requirements of the module are not satisfied: current kubernetes version is not suitable", slog.String("name", name), log.Err(err))
		return ptr.To(false), fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debug("requirements of module are satisfied", slog.String("name", name))
	return ptr.To(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		if name != "" {
			e.logger.Debug("requirements of the module are not satisfied, kubernetes version is not suitable", slog.String("name", name), slog.String("version", baseVersion), log.Err(err))
			return name, fmt.Errorf("requirements of the '%s' module are not satisfied: %s kubernetes version is not suitable: %s", name, baseVersion, err.Error())
		}
		e.logger.Debug("requirements cannot be checked: kubernetes version is invalid", slog.String("version", err.Error()))
		return "", fmt.Errorf("requirements cannot be checked: kubernetes version is invalid: %s", err.Error())
	}
	e.logger.Debug("modules requirements for kubernets version are satisfied", slog.String("version", baseVersion))
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
	e.logger.Debug("validate requirements", slog.String("name", releaseName))
	if err := e.versionMatcher.ValidateConstraint(rawConstraint); err != nil {
		e.logger.Debug("requirements of the module release are not satisfied: current kubernetes version is not suitable", slog.String("name", releaseName), log.Err(err))
		return fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debug("requirements of the module release are satisfied", slog.String("name", releaseName))
	return nil
}
