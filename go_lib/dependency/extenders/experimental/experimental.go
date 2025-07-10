/*
Copyright 2025 Flant JSC

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
package experimental

import (
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// Name is the identifier of the extender returned to the scheduler
	Name extenders.ExtenderName = "Experimental"
)

type Extender struct {
	allowExperimental bool
	logger            *log.Logger
	modules           map[string]struct{}
}

// Instance returns a singleton extender (same pattern as other extenders)
func NewExtender(allowExperimental bool, logger *log.Logger) *Extender {
	return &Extender{
		logger:            logger,
		allowExperimental: allowExperimental,
		modules:           make(map[string]struct{}),
	}
}

// Name returns the extender identifier
func (e *Extender) Name() extenders.ExtenderName { return Name }

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool { return true }

// AddConstraint configures the cluster‑wide flag. The scheduler is expected
// to call it once (for example when parsing Deckhouse values)
func (e *Extender) AddConstraint(name string) {
	e.modules[name] = struct{}{}
}

// DeleteConstraint is a no‑op for this extender (flag remains unchanged)
func (e *Extender) DeleteConstraint(name string) {
	delete(e.modules, name)
}

// Filter blocks installation of modules with `stage: Experimental` unless the
// flag allowExperimentalModules is true.
//
// If the module stage is not Experimental - pass (return nil, nil)
// If the stage is Experimental and the flag is false - deny with an error
// If the stage is Experimental and the flag is true  - allow (true, nil)
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	if e.allowExperimental {
		e.logger.Debug("experimental modules allowed", slog.String("name", name))
		return ptr.To(true), nil
	}

	if _, ok := e.modules[name]; ok {
		e.logger.Error("experimental module is forbidden by policy", slog.String("name", name))
		return ptr.To(false), &scherror.PermanentError{Err: fmt.Errorf("requirements are not satisfied: experimental modules are disabled (allowExperimentalModules=false)")}
	}

	return nil, nil
}
