// Copyright 2024 Flant JSC
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

package types

import (
	"fmt"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Module struct {
	def   *Definition
	basic *addonmodules.BasicModule
}

func NewModule(def *Definition, static addonutils.Values, config, values []byte, logger *log.Logger) (*Module, error) {
	logOpt := addonmodules.WithLogger(logger)
	basic, err := addonmodules.NewBasicModule(def.Name, def.Path, def.Weight, static, config, values, logOpt)
	if err != nil {
		return nil, fmt.Errorf("build the '%s' basic module: %w", def.Name, err)
	}

	basic.SetCritical(def.Critical)

	return &Module{
		def:   def,
		basic: basic,
	}, nil
}

func (m *Module) GetBasicModule() *addonmodules.BasicModule {
	if m == nil {
		return nil
	}

	return m.basic
}

func (m *Module) GetModuleDefinition() *Definition {
	return m.def
}

func (m *Module) GetModuleExclusiveGroup() *string {
	if m.def.ExclusiveGroup == "" {
		return nil
	}
	return &m.def.ExclusiveGroup
}

func (m *Module) GetConfirmationDisableReason() (string, bool) {
	if m.def != nil && m.def.DisableOptions != nil {
		return m.def.DisableOptions.Message, m.def.DisableOptions.Confirmation
	}
	return "", false
}
