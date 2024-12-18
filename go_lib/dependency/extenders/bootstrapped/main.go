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

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name              extenders.ExtenderName = "Bootstrapped"
	RequirementsField string                 = "bootstrapped"
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
		e.logger.Debugf("adding installed constraint for the '%s' module failed", name)
		return err
	}
	e.modules[name] = parsed
	e.logger.Debugf("installed constraint for the '%s' module is added", name)
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debugf("deleting installed constraint for the '%s' module", name)
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
		bootstrapped, err := e.isBootstrapped("/tmp/cluster-is-bootstrapped")
		if err != nil {
			return nil, &scherror.PermanentError{Err: fmt.Errorf("failed to define bootstrapped: %s", err)}
		}
		if bootstrapped {
			e.logger.Debugf("requirements of the '%s' module are satisfied", name)
			return ptr.To(true), nil
		}
		e.logger.Errorf("requirements of the '%s' module are not satisfied: module requires the cluster to be bootstrapped", name)
		return ptr.To(false), fmt.Errorf("requirements are not satisfied: module requires the cluster to be bootstrapped")
	}
	return nil, nil
}

func (e *Extender) isBootstrapped(path string) (bool, error) {
	if val := os.Getenv("TEST_EXTENDER_BOOTSTRAPPED"); val != "" {
		instance.logger.Debugf("setting bootstrapped from env")
		parsed, err := strconv.ParseBool(val)
		if err == nil {
			return parsed, nil
		}
		instance.logger.Errorf("parse boostrapped from env failed: %v", err)
	}
	_, err := os.Stat(path)
	if err == nil {
		e.logger.Debugf("file %s exists, cluster is bootstrapped", path)
		return true, nil
	} else if os.IsNotExist(err) {
		e.logger.Debugf("file %s does not exist, cluster is not bootstrapped", path)
		return false, nil
	}
	e.logger.Errorf("failed to read file %s: %v", path, err)
	return false, err
}
