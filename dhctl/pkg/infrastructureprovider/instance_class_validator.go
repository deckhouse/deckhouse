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

package infrastructureprovider

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	yamlvalidation "github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const instanceClassAPIGroup = "deckhouse.io"

var instanceClassKindRegexp = regexp.MustCompile(`.+InstanceClass$`)

type InstanceClassValidator struct {
	metaConfig *config.MetaConfig
}

func NewInstanceClassValidator(metaConfig *config.MetaConfig) *InstanceClassValidator {
	return &InstanceClassValidator{
		metaConfig: metaConfig,
	}
}

func (v *InstanceClassValidator) InstanceClasses() ([]unstructured.Unstructured, error) {
	if v == nil || v.metaConfig == nil {
		return nil, fmt.Errorf("metaConfig must not be nil")
	}

	if strings.TrimSpace(v.metaConfig.ResourcesYAML) == "" {
		return nil, nil
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(v.metaConfig.ResourcesYAML), -1)
	instanceClasses := make([]unstructured.Unstructured, 0, len(docs))

	for i, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		index, err := yamlvalidation.ParseIndex(strings.NewReader(doc))
		if err != nil {
			return nil, fmt.Errorf("parse resources document %d index: %w", i, err)
		}

		if index.Group() != instanceClassAPIGroup || !instanceClassKindRegexp.MatchString(index.Kind) {
			continue
		}

		var resource unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &resource); err != nil {
			return nil, fmt.Errorf("unmarshal instance class from resources document %d: %w", i, err)
		}

		instanceClasses = append(instanceClasses, resource)
	}
	return instanceClasses, nil
}

func (v *InstanceClassValidator) ProviderName() string {
	if v == nil || v.metaConfig == nil {
		return ""
	}

	return v.metaConfig.ProviderName
}

func (v *InstanceClassValidator) ValidateProviderInstanceClasses() error {
	providerName := strings.ToLower(v.ProviderName())

	instanceClasses, err := v.InstanceClasses()
	if err != nil {
		return err
	}

	providerInstanceClassRegexp := regexp.MustCompile(fmt.Sprintf("^%sinstanceclass$", regexp.QuoteMeta(providerName)))

	for _, instanceClass := range instanceClasses {
		instanceClassKind := strings.ToLower(instanceClass.GetKind())
		if providerInstanceClassRegexp.MatchString(instanceClassKind) {
			continue
		}

		return fmt.Errorf("instance class %q does not match provider %q", instanceClass.GetKind(), providerName)
	}

	return nil
}

func (v *InstanceClassValidator) Validate(_ *unstructured.Unstructured) error {
	// TODO validate instance class fields
	return nil
}
