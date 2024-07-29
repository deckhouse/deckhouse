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
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"
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
	logger  logger.Logger
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:  log.WithField("extender", Name),
			modules: make(map[string]bool),
		}
	})
	return instance
}

func (e *Extender) AddInstalledConstraint(name string, value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		e.logger.Errorf("failed to parse expression %s: %v", name, err)
		return err
	}
	e.modules[name] = parsed
	e.logger.Debugf("bootstrapped installed constraint for %q is added", name)
	return nil
}

func (e *Extender) DeleteConstraints(name string) {
	e.logger.Debugf("deleting constraints for %q", name)
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
	// module requirement is true by default
	req := true
	if val, ok := e.modules[name]; ok {
		req = val
	}
	if req {
		bootstrapped, err := e.isBootstrapped("/tmp/cluster-is-bootstrapped")
		if err != nil {
			return nil, &scherror.PermanentError{Err: fmt.Errorf("parse bootstrapped file failed: %s", err)}
		}
		if bootstrapped {
			e.logger.Debugf("requirements of %s are satisfied", name)
			return pointer.Bool(true), nil
		}
		e.logger.Errorf("requirements of %s are not satisfied: module requires the cluster to be bootstrapped", name)
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: module requires the cluster to be bootstrapped")
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
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
