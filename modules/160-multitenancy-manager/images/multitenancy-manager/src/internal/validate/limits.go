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

package validate

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
)

const (
	configLimitsName      = "multitenancy-limits"
	configLimitsNamespace = "d8-multitenancy-manager"

	keyTemplateLimits = "templateLimits"
)

type limits struct {
	ProjectsLimit   int `json:"projectsLimit"`
	NamespacesLimit int `json:"namespacesLimit"`
}

func ProjectLimits(ctx context.Context, cli client.Client, project *v1alpha2.Project) error {
	config := new(corev1.ConfigMap)
	if err := cli.Get(ctx, client.ObjectKey{Name: configLimitsName, Namespace: configLimitsNamespace}, config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get the '%s/%s' config: %w", configLimitsNamespace, configLimitsName, err)
	}

	parsed := make(map[string]limits)
	if templateLimits, ok := config.Data[keyTemplateLimits]; ok {
		if err := yaml.Unmarshal([]byte(templateLimits), &parsed); err != nil {
			return fmt.Errorf("unmarshal template limits: %w", err)
		}

		projectsNumber, err := projectsNumberByTemplateName(ctx, cli, project)
		if err != nil {
			return fmt.Errorf("get projects number: %w", err)
		}

		if template, ok := parsed[project.Spec.ProjectTemplateName]; ok {
			if len(project.Spec.Namespaces) > template.NamespacesLimit {
				return errors.New("namespaces limit exceeded")
			}

			if projectsNumber+1 > template.ProjectsLimit {
				return errors.New("projects limit exceeded")
			}
		}
	}

	return nil
}

func projectsNumberByTemplateName(ctx context.Context, cli client.Client, project *v1alpha2.Project) (int, error) {
	projects := new(v1alpha2.ProjectList)
	if err := cli.List(ctx, projects, client.MatchingLabels{
		v1alpha2.ResourceLabelTemplate: project.Spec.ProjectTemplateName,
	}); err != nil {
		return 0, fmt.Errorf("list projects: %w", err)
	}
	return len(projects.Items), nil
}
