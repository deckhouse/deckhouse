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

package ensure_rbacv2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryYaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// installer simultaneously installs rbacv2 from specified directory
type installer struct {
	client   k8s.Client
	crdsDirs [][]string
	buffer   []byte

	moduleName  string
	moduleScope string

	// concurrent tasks to create resource in a k8s cluster
	tasks *multierror.Group
}

// newInstaller creates new installer for CRDs
// pathToCRDs example: "/deckhouse/modules/002-deckhouse/crds/*.yaml"
func newInstaller(moduleName, moduleScope string, client k8s.Client, pathsToCRDs []string) (*installer, error) {
	var crdsDirs [][]string
	for _, dir := range pathsToCRDs {
		crds, err := filepath.Glob(dir)
		if err != nil {
			return nil, err
		}
		crdsDirs = append(crdsDirs, crds)
	}
	return &installer{
		client:   client,
		crdsDirs: crdsDirs,

		moduleName:  moduleName,
		moduleScope: moduleScope,
		// 1Mb - maximum size of kubernetes object
		// if we take less, we have to handle io.ErrShortBuffer error and increase the buffer
		// take more does not make any sense due to kubernetes limitations
		buffer: make([]byte, 1*1024*1024),
		tasks:  &multierror.Group{},
	}, nil
}

func (i *installer) Run(ctx context.Context) *multierror.Error {
	crds, err := i.parseCRDs(ctx)
	if err != nil {
		return multierror.Append(&multierror.Error{}, err)
	}
	return i.ensureRoles(ctx, crds)
}

func (i *installer) parseCRDs(_ context.Context) ([]*v1.CustomResourceDefinition, error) {
	var crds []*v1.CustomResourceDefinition
	for _, dir := range i.crdsDirs {
		for _, pathToCRD := range dir {
			if match := strings.HasPrefix(filepath.Base(pathToCRD), "doc-"); match {
				continue
			}
			crd, err := i.parseCRD(pathToCRD)
			if err != nil {
				return nil, err
			}
			crds = append(crds, crd)
		}
	}
	return crds, nil
}
func (i *installer) parseCRD(path string) (*v1.CustomResourceDefinition, error) {
	var crd *v1.CustomResourceDefinition
	fileReader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()
	reader := apimachineryYaml.NewDocumentDecoder(fileReader)
	for {
		bufferSize, err := reader.Read(i.buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		data := i.buffer[:bufferSize]
		if len(data) == 0 {
			// some empty yaml document, or empty string before separator
			continue
		}
		if err = apimachineryYaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), bufferSize).Decode(&crd); err != nil {
			return nil, err
		}
		// it could be a comment or some other peace of yaml file, skip it
		if crd == nil {
			return nil, nil
		}
		if crd.APIVersion != v1.SchemeGroupVersion.String() && crd.Kind != "CustomResourceDefinition" {
			return nil, fmt.Errorf("invalid CRD document apiversion/kind: '%s/%s'", crd.APIVersion, crd.Kind)
		}
	}
	return crd, nil
}

func (i *installer) ensureRoles(ctx context.Context, crds []*v1.CustomResourceDefinition) *multierror.Error {
	namespacedView, namespacedEdit, view, edit := i.clusterRoles(crds)
	if namespacedView != nil {
		i.tasks.Go(func() error {
			return i.ensureRole(ctx, namespacedView)
		})
	}
	if namespacedEdit != nil {
		i.tasks.Go(func() error {
			return i.ensureRole(ctx, namespacedEdit)
		})
	}
	if view != nil {
		i.tasks.Go(func() error {
			return i.ensureRole(ctx, view)
		})
	}
	if edit != nil {
		i.tasks.Go(func() error {
			return i.ensureRole(ctx, edit)
		})
	}
	if errs := i.tasks.Wait(); errs.ErrorOrNil() != nil {
		return multierror.Append(&multierror.Error{}, errs.Errors...)
	}
	return nil
}
func (i *installer) ensureRole(ctx context.Context, role *rbacv1.ClusterRole) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		found, err := i.client.RbacV1().ClusterRoles().Get(ctx, role.Name, apimachineryv1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = i.client.RbacV1().ClusterRoles().Create(ctx, role, apimachineryv1.CreateOptions{})
			}
			return err
		}
		if reflect.DeepEqual(role.Rules, found.Rules) {
			return nil
		}
		_, err = i.client.RbacV1().ClusterRoles().Update(ctx, role, apimachineryv1.UpdateOptions{})
		return err
	})
}

func (i *installer) clusterRoles(crds []*v1.CustomResourceDefinition) (*rbacv1.ClusterRole, *rbacv1.ClusterRole, *rbacv1.ClusterRole, *rbacv1.ClusterRole) {
	var namespacedViewRules, namespacedEditRules, viewRules, editRules []rbacv1.PolicyRule
	for _, crd := range crds {
		viewRule := rbacv1.PolicyRule{
			APIGroups: []string{crd.Spec.Group},
			Resources: []string{crd.Spec.Names.Plural},
			Verbs:     []string{"get", "list", "watch"},
		}
		editRule := rbacv1.PolicyRule{
			APIGroups: []string{crd.Spec.Group},
			Resources: []string{crd.Spec.Names.Plural},
			Verbs:     []string{"create", "update", "path", "delete", "deletecollection"},
		}
		if crd.Spec.Scope == "Cluster" {
			viewRules = append(viewRules, viewRule)
			editRules = append(editRules, editRule)
		} else {
			namespacedViewRules = append(namespacedViewRules, viewRule)
			namespacedEditRules = append(namespacedEditRules, editRule)
		}
	}
	var namespacedViewClusterRole *rbacv1.ClusterRole
	if namespacedViewRules != nil {
		namespacedViewClusterRole = i.clusterRolesFromRules("viewer", "use", "view", namespacedViewRules)
	}
	var namespacedEditClusterRole *rbacv1.ClusterRole
	if namespacedEditRules != nil {
		namespacedEditClusterRole = i.clusterRolesFromRules("manager", "use", "edit", namespacedEditRules)
	}

	viewRules = append(viewRules, rbacv1.PolicyRule{
		APIGroups:     []string{"deckhouse.io"},
		Resources:     []string{"moduleconfigs"},
		ResourceNames: []string{i.moduleName},
		Verbs:         []string{"get", "list", "watch"},
	})
	editRules = append(editRules, rbacv1.PolicyRule{
		APIGroups:     []string{"deckhouse.io"},
		Resources:     []string{"moduleconfigs"},
		ResourceNames: []string{i.moduleName},
		Verbs:         []string{"create", "update", "patch", "delete"},
	})

	viewClusterRole := i.clusterRolesFromRules("viewer", "manage", "view", viewRules)
	editClusterRole := i.clusterRolesFromRules("manager", "manage", "edit", editRules)

	return namespacedViewClusterRole, namespacedEditClusterRole, viewClusterRole, editClusterRole
}
func (i *installer) clusterRolesFromRules(rbacRole, rbacKind, rbacVerb string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	role := &rbacv1.ClusterRole{
		TypeMeta: apimachineryv1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: apimachineryv1.ObjectMeta{
			Name: fmt.Sprintf("d8:%s:capability:module:%s:%s", rbacKind, i.moduleName, rbacVerb),
			Labels: map[string]string{
				"rbac.deckhouse.io/kind":              rbacKind,
				"rbac.deckhouse.io/aggregate-to-role": rbacRole,
			},
		},
		Rules: rules,
	}
	if rbacKind == "manage" {
		role.ObjectMeta.Labels["rbac.deckhouse.io/aggregate-to-scope"] = i.moduleScope
	}
	return role
}
