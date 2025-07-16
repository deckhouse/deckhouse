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

package bootstrapped

import (
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	exterr "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"
)

const (
	Name extenders.ExtenderName = "Bootstrapped"
)

type Extender struct {
	// check if the cluster bootstrapped
	isBootstrapped func() (bool, error)
	// functional modules require bootstrapped cluster
	modules map[string]struct{}
}

func NewExtender(helper func() (bool, error)) *Extender {
	return &Extender{
		modules:        make(map[string]struct{}),
		isBootstrapped: helper,
	}
}

func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

func (e *Extender) Filter(moduleName string, _ map[string]string) (*bool, error) {
	if _, ok := e.modules[moduleName]; ok {
		bootstrapped, err := e.isBootstrapped()
		if err != nil {
			return nil, exterr.Permanent(err)
		}

		// enable functional modules only if the cluster bootstrapped
		return ptr.To(bootstrapped), nil
	}

	return nil, nil
}

func (e *Extender) IsTerminator() bool {
	return true
}

func (e *Extender) AddFunctionalModule(moduleName string) {
	e.modules[moduleName] = struct{}{}
}
