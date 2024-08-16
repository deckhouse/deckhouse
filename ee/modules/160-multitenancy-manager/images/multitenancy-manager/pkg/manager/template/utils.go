/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	"controller/pkg/validate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (m *manager) ensureDefaultProjectTemplates(ctx context.Context, templatesPath string) error {
	dir, err := os.ReadDir(templatesPath)
	if err != nil {
		m.log.Error(err, "unable to read directory", "directory", templatesPath)
		return err
	}
	for _, file := range dir {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			m.log.Info("skipping file as it's not a YAML file", "file", file.Name())
			continue
		}
		m.log.Info("reading file with project template", "file", file.Name())
		projectTemplateBytes, err := os.ReadFile(filepath.Join(templatesPath, file.Name()))
		if err != nil {
			m.log.Error(err, "failed to read project template", "file", file.Name())
			return err
		}
		projectTemplate := new(v1alpha1.ProjectTemplate)
		if err = yaml.Unmarshal(projectTemplateBytes, projectTemplate); err != nil {
			m.log.Error(err, "failed to unmarshal project", "file", file.Name())
			return err
		}
		m.log.Info("validating project template", "file", file.Name())
		if err = validate.ProjectTemplate(projectTemplate); err != nil {
			m.log.Error(err, "invalid project template", "file", file.Name())
			return err
		}
		m.log.Info("creating project template", "file", file.Name())
		if err = m.client.Create(ctx, projectTemplate); err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
					existingProjectTemplate := new(v1alpha1.ProjectTemplate)
					if err = m.client.Get(ctx, types.NamespacedName{Name: projectTemplate.Name}, existingProjectTemplate); err != nil {
						m.log.Error(err, "failed to fetch project template", "file", file.Name())
						return err
					}
					existingProjectTemplate.Spec = projectTemplate.Spec
					m.log.Info("project template already exists, try to update it", "file", file.Name())
					if err = m.client.Update(ctx, existingProjectTemplate); err != nil {
						m.log.Error(err, "failed to update project template", "file", file.Name())
						return err
					}
					return nil
				})
				if err != nil {
					return err
				}
			} else {
				m.log.Error(err, "failed to create project template", "file", file.Name())
				return err
			}
		}
		m.log.Info("successfully ensured project template", "file", file.Name())
	}
	return nil
}

func (m *manager) setTemplateStatus(ctx context.Context, template *v1alpha1.ProjectTemplate, message string, ready bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: template.Name}, template); err != nil {
			return err
		}
		template.Status.Message = message
		template.Status.Ready = ready
		return m.client.Status().Update(ctx, template)
	})
}

func (m *manager) projectsByTemplate(ctx context.Context, template *v1alpha1.ProjectTemplate) ([]*v1alpha2.Project, error) {
	projects := new(v1alpha2.ProjectList)
	if err := m.client.List(ctx, projects, client.MatchingLabels{helm.ProjectTemplateLabel: template.Name}); err != nil {
		return nil, err
	}
	if len(projects.Items) == 0 {
		return nil, nil
	}
	var result []*v1alpha2.Project
	for _, project := range projects.Items {
		if project.Status.State == v1alpha2.ProjectStateDeployed || project.Status.State == v1alpha2.ProjectStateError {
			result = append(result, project.DeepCopy())
			continue
		}
		m.log.Info("skipping not deployed project", "project", project.Name)
	}
	return result, nil
}
