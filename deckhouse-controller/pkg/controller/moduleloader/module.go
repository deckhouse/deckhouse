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

package moduleloader

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	shapp "github.com/flant/shell-operator/pkg/app"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Module struct {
	basic *modules.BasicModule

	description string
	stage       string
	labels      map[string]string

	needConfirmDisable        bool
	needConfirmDisableMessage string
}

func newModule(def *Definition, staticValues utils.Values, configBytes, valuesBytes []byte, logger *log.Logger) (*Module, error) {
	basic, err := modules.NewBasicModule(def.Name, def.Path, def.Weight, staticValues, configBytes, valuesBytes, app.CRDsFilters, shapp.DebugKeepTmpFiles, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build the '%s' basic module: %w", def.Name, err)
	}

	labels := make(map[string]string, len(def.Tags))
	for _, tag := range def.Tags {
		labels["module.deckhouse.io/"+tag] = ""
	}

	if len(def.Tags) == 0 {
		labels = calculateLabels(def.Name)
	}

	return &Module{
		basic:                     basic,
		labels:                    labels,
		description:               def.Description,
		stage:                     def.Stage,
		needConfirmDisable:        def.DisableOptions.Confirmation,
		needConfirmDisableMessage: def.DisableOptions.Message,
	}, nil
}

func (m *Module) GetBasicModule() *modules.BasicModule {
	return m.basic
}

func (m *Module) GetConfirmationDisableReason() (string, bool) {
	return m.needConfirmDisableMessage, m.needConfirmDisable
}

func calculateLabels(name string) map[string]string {
	// could be removed when we will ready properties from the module.yaml file
	labels := make(map[string]string, 0)

	if strings.HasPrefix(name, "cni-") {
		labels["module.deckhouse.io/cni"] = ""
	}

	if strings.HasPrefix(name, "cloud-provider-") {
		labels["module.deckhouse.io/cloud-provider"] = ""
	}

	if strings.HasSuffix(name, "-crd") {
		labels["module.deckhouse.io/crd"] = ""
	}

	return labels
}
