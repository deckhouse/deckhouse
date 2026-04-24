// Copyright 2026 Flant JSC
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

package template

import (
	"context"
	"fmt"
	"path/filepath"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func prepareNodeGroupConfigurationSteps(
	ctx context.Context,
	templateController *Controller,
	resourcesYAML string,
	templateData map[string]interface{},
) error {
	ngc, err := config.ParseInternalBootstrapNodeGroupConfiguration(ctx, resourcesYAML)
	if err != nil {
		return fmt.Errorf("parse NodeGroupConfigurations: %w", err)
	}

	if ngc == nil {
		return nil
	}

	weight := deckhousev1alpha1.NodeGroupConfigurationDefaultWeight
	if ngc.Spec.Weight != nil {
		weight = *ngc.Spec.Weight
	}

	templateName := fmt.Sprintf("%03d_%s", weight, ngc.Name)
	rendered, err := RenderTemplate(templateName, []byte(ngc.Spec.Content), templateData)
	if err != nil {
		return fmt.Errorf("render NodeGroupConfiguration %q: %w", ngc.Name, err)
	}

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	return SaveRenderedToDir([]RenderedTemplate{*rendered}, stepsPath)
}
