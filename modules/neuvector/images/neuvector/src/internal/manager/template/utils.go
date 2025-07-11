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

package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/validate"
)

func (m *Manager) ensureDefaultProjectTemplates(ctx context.Context, templatesPath string) error {
	dir, err := os.ReadDir(templatesPath)
	if err != nil {
		return fmt.Errorf("read the '%s' directory: %w", templatesPath, err)
	}

	for _, file := range dir {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			m.logger.Info("skip file as it's not a YAML file", "file", file.Name())
			continue
		}

		m.logger.Info("read the file with project template", "file", file.Name())
		projectTemplateBytes, err := os.ReadFile(filepath.Join(templatesPath, file.Name()))
		if err != nil {
			return fmt.Errorf("read the '%s' project template file: %w", file.Name(), err)
		}

		projectTemplate := new(v1alpha1.ProjectTemplate)
		if err = yaml.Unmarshal(projectTemplateBytes, projectTemplate); err != nil {
			return fmt.Errorf("unmarshal the '%s' project template file: %w", file.Name(), err)
		}

		m.logger.Info("validate project template", "file", file.Name())
		if err = validate.ProjectTemplate(projectTemplate); err != nil {
			return fmt.Errorf("'%s' invalid project template file: %w", file.Name(), err)
		}

		if err = m.ensureProjectTemplate(ctx, projectTemplate); err != nil {
			return fmt.Errorf("ensure '%s' project template: %w", file.Name(), err)
		}
	}

	return nil
}

func (m *Manager) ensureProjectTemplate(ctx context.Context, projectTemplate *v1alpha1.ProjectTemplate) error {
	m.logger.Info("ensure project template", "projectTemplate", projectTemplate.Name)
	if err := m.client.Create(ctx, projectTemplate); err != nil {
		if apierrors.IsAlreadyExists(err) {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingProjectTemplate := new(v1alpha1.ProjectTemplate)
				if err = m.client.Get(ctx, client.ObjectKey{Name: projectTemplate.Name}, existingProjectTemplate); err != nil {
					return fmt.Errorf("get the '%s' project template: %w", projectTemplate.Name, err)
				}

				existingProjectTemplate.Spec = projectTemplate.Spec
				existingProjectTemplate.Labels = projectTemplate.Labels
				existingProjectTemplate.Annotations = projectTemplate.Annotations

				m.logger.Info("project template already exists, try to update it", "projectTemplate", projectTemplate.Name)
				return m.client.Update(ctx, existingProjectTemplate)
			})
			if err != nil {
				return fmt.Errorf("update the '%s' project template: %w", projectTemplate.Name, err)
			}
		} else {
			return fmt.Errorf("create the '%s' project template: %w", projectTemplate.Name, err)
		}
	}

	m.logger.Info("the project template ensured", "projectTemplate", projectTemplate.Name)
	return nil
}

func (m *Manager) setTemplateStatus(ctx context.Context, template *v1alpha1.ProjectTemplate, message string, ready bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, client.ObjectKey{Name: template.Name}, template); err != nil {
			return fmt.Errorf("get the '%s' project template: %w", template.Name, err)
		}
		template.Status.Message = message
		template.Status.Ready = ready
		return m.client.Status().Update(ctx, template)
	})
}

func (m *Manager) projectsByTemplate(ctx context.Context, template *v1alpha1.ProjectTemplate) ([]*v1alpha2.Project, error) {
	projects := new(v1alpha2.ProjectList)
	if err := m.client.List(ctx, projects, client.MatchingLabels{v1alpha2.ResourceLabelTemplate: template.Name}); err != nil {
		return nil, fmt.Errorf("list projects by template: %w", err)
	}
	if len(projects.Items) == 0 {
		return nil, nil
	}
	var result []*v1alpha2.Project
	for _, project := range projects.Items {
		result = append(result, project.DeepCopy())
	}
	return result, nil
}
