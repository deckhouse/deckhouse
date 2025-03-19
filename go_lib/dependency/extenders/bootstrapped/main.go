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

package bootstrapped

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "Bootstrapped"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	modules map[string]bool
	logger  *log.Logger
}

// TODO: refactor
func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:  log.Default().With("extender", Name),
			modules: make(map[string]bool),
		}
	})
	return instance
}

func (e *Extender) AddConstraint(name string, value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		e.logger.Debug("adding installed constraint for module failed", slog.String("name", name))
		return err
	}

	e.modules[name] = parsed
	e.logger.Debug("installed constraint for module is added", slog.String("name", name))

	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debug("deleting installed constraint for module", slog.String("name", name))
	delete(e.modules, name)
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
	if req, ok := e.modules[name]; ok {
		if !req {
			e.logger.Debug("module does not require the cluster to be boostrapped", slog.String("name", name))
			return nil, nil
		}
		bootstrapped, err := e.isBootstrapped("/tmp/cluster-is-bootstrapped")
		if err != nil {
			return nil, &scherror.PermanentError{Err: fmt.Errorf("failed to define bootstrapped: %s", err)}
		}
		if bootstrapped {
			e.logger.Debug("requirements of module are satisfied", slog.String("name", name))
			return ptr.To(true), nil
		}
		e.logger.Error("requirements of the module are not satisfied: module requires the cluster to be bootstrapped", slog.String("name", name))
		return ptr.To(false), fmt.Errorf("requirements are not satisfied: module requires the cluster to be bootstrapped")
	}
	return nil, nil
}

func (e *Extender) isBootstrapped(path string) (bool, error) {
	if val := os.Getenv("TEST_EXTENDER_BOOTSTRAPPED"); val != "" {
		instance.logger.Debug("setting bootstrapped from env")
		parsed, err := strconv.ParseBool(val)
		if err == nil {
			return parsed, nil
		}
		instance.logger.Error("parse boostrapped from env failed", log.Err(err))
	}
	_, err := os.Stat(path)
	if err == nil {
		e.logger.Debug("file exists, cluster is bootstrapped", slog.String("path", path))
		return true, nil
	} else if os.IsNotExist(err) {
		e.logger.Debug("file does not exist, cluster is not bootstrapped", slog.String("path", path))
		return false, nil
	}
	e.logger.Error("failed to read file", slog.String("path", path), log.Err(err))
	return false, err
}
