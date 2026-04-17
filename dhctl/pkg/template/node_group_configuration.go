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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const nodeGroupConfigurationBundleHeader = `
### Auto-generated NGC header start ###
case $(bb-is-bundle) in
	%s) ;;
	*) exit 0 ;;
esac
### Auto-generated NGC header end ###
`

const (
	bootstrapNodeGroupName              = "master"
	bootstrapNodeGroupConfigurationName = "d8-early-node-bootstrap-internal.sh"
)

func prepareNodeGroupConfigurationSteps(
	ctx context.Context,
	templateController *Controller,
	resourcesYAML string,
	templateData map[string]interface{},
) error {
	nodeGroupConfigurations, err := config.ParseNodeGroupConfigurations(ctx, resourcesYAML)
	if err != nil {
		return fmt.Errorf("parse NodeGroupConfigurations: %w", err)
	}

	renderedTemplates, err := renderNodeGroupConfigurationSteps(nodeGroupConfigurations, templateData)
	if err != nil {
		return err
	}

	if len(renderedTemplates) == 0 {
		return nil
	}

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	renderedTemplates, err = filterNodeGroupConfigurationStepConflicts(renderedTemplates, stepsPath)
	if err != nil {
		return err
	}

	if len(renderedTemplates) == 0 {
		return nil
	}

	return SaveRenderedToDir(renderedTemplates, stepsPath)
}

func renderNodeGroupConfigurationSteps(
	nodeGroupConfigurations []deckhousev1alpha1.NodeGroupConfiguration,
	templateData map[string]interface{},
) ([]RenderedTemplate, error) {
	renderedTemplates := make([]RenderedTemplate, 0, len(nodeGroupConfigurations))

	for _, nodeGroupConfiguration := range nodeGroupConfigurations {
		if !nodeGroupConfigurationAppliesToBootstrapMaster(nodeGroupConfiguration) {
			continue
		}

		templateName := fmt.Sprintf("%03d_%s", nodeGroupConfigurationWeight(nodeGroupConfiguration), nodeGroupConfiguration.Name)
		renderedTemplate, err := RenderTemplate(templateName, []byte(nodeGroupConfiguration.Spec.Content), templateData)
		if err != nil {
			return nil, fmt.Errorf("render NodeGroupConfiguration %q: %w", nodeGroupConfiguration.Name, err)
		}

		if !nodeGroupConfigurationHasWildcardBundle(nodeGroupConfiguration) {
			header := fmt.Sprintf(nodeGroupConfigurationBundleHeader, strings.Join(nodeGroupConfiguration.Spec.Bundles, "|"))
			renderedTemplate.Content = bytes.NewBufferString(fmt.Sprintf("%s\n%s", header, renderedTemplate.Content.String()))
		}

		renderedTemplates = append(renderedTemplates, *renderedTemplate)
	}

	sort.Slice(renderedTemplates, func(i, j int) bool {
		return renderedTemplates[i].FileName < renderedTemplates[j].FileName
	})

	return renderedTemplates, nil
}

func filterNodeGroupConfigurationStepConflicts(renderedTemplates []RenderedTemplate, stepsPath string) ([]RenderedTemplate, error) {
	existingSteps := make(map[string]struct{})
	files, err := os.ReadDir(stepsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read bashible steps directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		existingSteps[file.Name()] = struct{}{}
	}

	filteredTemplates := make([]RenderedTemplate, 0, len(renderedTemplates))
	for _, renderedTemplate := range renderedTemplates {
		if _, exists := existingSteps[renderedTemplate.FileName]; exists {
			log.WarnF("NodeGroupConfiguration step %q conflicts with existing bashible step. Skip!\n", renderedTemplate.FileName)
			continue
		}

		existingSteps[renderedTemplate.FileName] = struct{}{}
		filteredTemplates = append(filteredTemplates, renderedTemplate)
	}

	return filteredTemplates, nil
}

func nodeGroupConfigurationAppliesToBootstrapMaster(nodeGroupConfiguration deckhousev1alpha1.NodeGroupConfiguration) bool {
	if nodeGroupConfiguration.Name != bootstrapNodeGroupConfigurationName {
		return false
	}

	for _, configuredNodeGroupName := range nodeGroupConfiguration.Spec.NodeGroups {
		if configuredNodeGroupName == "*" || configuredNodeGroupName == bootstrapNodeGroupName {
			return true
		}
	}

	return false
}

func nodeGroupConfigurationHasWildcardBundle(nodeGroupConfiguration deckhousev1alpha1.NodeGroupConfiguration) bool {
	for _, bundle := range nodeGroupConfiguration.Spec.Bundles {
		if bundle == "*" {
			return true
		}
	}

	return false
}

func nodeGroupConfigurationWeight(nodeGroupConfiguration deckhousev1alpha1.NodeGroupConfiguration) int {
	if nodeGroupConfiguration.Spec.Weight == nil {
		return deckhousev1alpha1.NodeGroupConfigurationDefaultWeight
	}

	return *nodeGroupConfiguration.Spec.Weight
}
