/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"controller/pkg/helm"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"slices"
	"strings"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/validate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
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

func (m *manager) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := m.client.Get(ctx, types.NamespacedName{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return template, nil
}

func (m *manager) setProjectStatus(ctx context.Context, project *v1alpha2.Project, state, message string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		project.Status.Message = message
		project.Status.State = state
		project.Status.Sync = false
		if state == v1alpha2.ProjectStateDeployed {
			project.Status.Sync = true
			if project.Status.Namespaces == nil {
				project.Status.Namespaces = []string{}
			}
			if !slices.Contains(project.Status.Namespaces, project.Name) {
				project.Status.Namespaces = append(project.Status.Namespaces, project.Name)
			}
		}
		return m.client.Status().Update(ctx, project)
	})
}

func (m *manager) setFinalizer(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		if project.Labels == nil {
			project.Labels = make(map[string]string)
		}
		project.Labels[helm.ProjectTemplateLabel] = project.Spec.ProjectTemplateName
		if project.Annotations != nil {
			delete(project.Annotations, helm.ProjectRequireSyncAnnotation)
		}
		if !controllerutil.ContainsFinalizer(project, Finalizer) {
			controllerutil.AddFinalizer(project, Finalizer)
		}
		return m.client.Update(ctx, project)
	})
}

func (m *manager) removeFinalizer(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		if !controllerutil.ContainsFinalizer(project, Finalizer) {
			return nil
		}
		controllerutil.RemoveFinalizer(project, Finalizer)
		return m.client.Update(ctx, project)
	})
}
