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
	"os"
	"strconv"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name              extenders.ExtenderName = "Bootstrapped"
	RequirementsField string                 = "bootstrapped"

	bootstrappedFile = "/tmp/cluster-is-bootstrapped"
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
		return fmt.Errorf("add constraint for the '%s' module: %w", name, err)
	}
	e.modules[name] = parsed
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
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
			e.logger.Debugf("the '%s' module does not require the cluster to be boostrapped", name)
			return nil, nil
		}

		bootstrapped, err := e.isBootstrapped(bootstrappedFile)
		if err != nil {
			return nil, &scherror.PermanentError{Err: fmt.Errorf("define bootstrapped: %w", err)}
		}

		if bootstrapped {
			e.logger.Debugf("the '%s' module`s requirements met", name)
			return ptr.To(true), nil
		}

		return ptr.To(false), fmt.Errorf("the '%s' module`s requirements not met: module requires the cluster to be bootstrapped", name)
	}

	return nil, nil
}

func (e *Extender) isBootstrapped(path string) (bool, error) {
	// try to set bootstrapped from env
	if val := app.TestVarExtenderBootstrapped; val != "" {
		instance.logger.Debugf("set bootstrapped from env")
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed, nil
		}
		e.logger.Debugf("failed to parse boostrapped from env")
	}

	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat the '%s' file: %w", path, err)
		}
		e.logger.Debugf("the '%s' file does not exist, cluster not bootstrapped", path)
		return false, nil
	}

	e.logger.Debugf("the '%s' file exists, cluster bootstrapped", path)
	return true, nil
}
