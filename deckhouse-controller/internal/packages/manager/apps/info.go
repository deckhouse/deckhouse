// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apps

import (
	addonutils "github.com/flant/addon-operator/pkg/utils"
)

type Info struct {
	Instance   string            `json:"name" yaml:"name"`
	Namespace  string            `json:"namespace" yaml:"namespace"`
	Definition Definition        `json:"definition" yaml:"definition"`
	Values     addonutils.Values `json:"values,omitempty" yaml:"values,omitempty"`
	Hooks      []string          `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

func (a *Application) GetInfo() Info {
	hooks := make([]string, len(a.hooks.GetHooks()))
	for idx, hook := range a.hooks.GetHooks() {
		hooks[idx] = hook.GetName()
	}

	return Info{
		Instance:   a.instance,
		Namespace:  a.namespace,
		Definition: a.definition,
		Values:     a.values.GetValues(),
		Hooks:      hooks,
	}
}
