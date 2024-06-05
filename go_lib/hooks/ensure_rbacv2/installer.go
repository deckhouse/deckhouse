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

var scopeTemplate = "rbac.deckhouse.io/aggregate-to-%s"

// installer simultaneously installs rbacv2 from specified directory
type installer struct {
	client   k8s.Client
	crdsDirs [][]string
	buffer   []byte

	moduleName   string
	moduleScopes []string

	// concurrent tasks to create resource in a k8s cluster
	tasks *multierror.Group
}

// newInstaller creates new installer for CRDs
// pathToCRDs example: "/deckhouse/modules/002-deckhouse/crds/*.yaml"
func newInstaller(moduleName string, moduleScopes []string, client k8s.Client, pathsToCRDs []string) (*installer, error) {
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

		moduleName:   moduleName,
		moduleScopes: moduleScopes,
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
	// check that module scopes exist, if they do not, ensure them
	if errs := i.ensureScopes(ctx); errs != nil {
		return errs
	}
	return i.ensureRoles(ctx, i.capabilitiesClusterRoles(crds))
}

func (i *installer) parseCRDs(ctx context.Context) ([]*v1.CustomResourceDefinition, error) {
	var crds []*v1.CustomResourceDefinition
	for _, dir := range i.crdsDirs {
		for _, pathToCRD := range dir {
			if match := strings.HasPrefix(filepath.Base(pathToCRD), "doc-"); match {
				continue
			}
			parsed, err := i.processFile(ctx, pathToCRD)
			if err != nil {
				return nil, err
			}
			if len(parsed) != 0 {
				crds = append(crds, parsed...)
			}
		}
	}
	return crds, nil
}
func (i *installer) processFile(ctx context.Context, path string) ([]*v1.CustomResourceDefinition, error) {
	fileReader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()
	var crds []*v1.CustomResourceDefinition
	reader := apimachineryYaml.NewDocumentDecoder(fileReader)
	for {
		n, err := reader.Read(i.buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		data := i.buffer[:n]
		if len(data) == 0 {
			// some empty yaml document, or empty string before separator
			continue
		}
		crd, err := i.parseCRD(ctx, bytes.NewReader(data), n)
		if err != nil {
			return nil, err
		}
		if crd != nil {
			crds = append(crds, crd)
		}
	}
	return crds, nil
}
func (i *installer) parseCRD(_ context.Context, reader io.Reader, bufferSize int) (*v1.CustomResourceDefinition, error) {
	var crd *v1.CustomResourceDefinition
	if err := apimachineryYaml.NewYAMLOrJSONDecoder(reader, bufferSize).Decode(&crd); err != nil {
		return nil, err
	}
	// it could be a comment or some other peace of yaml file, skip it
	if crd == nil {
		return nil, nil
	}
	if crd.APIVersion != v1.SchemeGroupVersion.String() && crd.Kind != "CustomResourceDefinition" {
		return nil, fmt.Errorf("invalid CRD document apiversion/kind: '%s/%s'", crd.APIVersion, crd.Kind)
	}
	if crd.Spec.Group != "deckhouse.io" {
		return nil, nil
	}
	return crd, nil
}

func (i *installer) capabilitiesClusterRoles(crds []*v1.CustomResourceDefinition) []*rbacv1.ClusterRole {
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
	//deckhouse can manage all module configs
	if i.moduleName != "deckhouse" {
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
	}
	var roles = []*rbacv1.ClusterRole{
		i.capabilityClusterRoleFromRules("viewer", "manage", "view", viewRules),
		i.capabilityClusterRoleFromRules("manager", "manage", "edit", editRules),
	}
	if namespacedViewRules != nil {
		roles = append(roles, i.capabilityClusterRoleFromRules("viewer", "use", "view", namespacedViewRules))
	}
	if namespacedEditRules != nil {
		roles = append(roles, i.capabilityClusterRoleFromRules("manager", "use", "edit", namespacedEditRules))
	}
	return roles
}
func (i *installer) capabilityClusterRoleFromRules(rbacRole, rbacKind, rbacVerb string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	role := &rbacv1.ClusterRole{
		TypeMeta: apimachineryv1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: apimachineryv1.ObjectMeta{
			Name: fmt.Sprintf("d8:%s:capability:module:%s:%s", rbacKind, i.moduleName, rbacVerb),
			Labels: map[string]string{
				"heritage":                            "deckhouse",
				"module":                              i.moduleName,
				"rbac.deckhouse.io/kind":              rbacKind,
				"rbac.deckhouse.io/aggregate-to-role": rbacRole,
			},
		},
		Rules: rules,
	}
	if rbacKind == "manage" {
		for _, scope := range i.moduleScopes {
			role.ObjectMeta.Labels[fmt.Sprintf(scopeTemplate, scope)] = "true"
		}
	}
	return role
}

func (i *installer) ensureScopes(ctx context.Context) *multierror.Error {
	var roles []*rbacv1.ClusterRole
	for _, scope := range i.moduleScopes {
		roles = append(roles, i.scopeClusterRole(scope, "viewer"))
		roles = append(roles, i.scopeClusterRole(scope, "manager"))
	}
	return i.ensureRoles(ctx, roles)
}
func (i *installer) scopeClusterRole(scope, role string) *rbacv1.ClusterRole {
	cr := &rbacv1.ClusterRole{
		TypeMeta: apimachineryv1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: apimachineryv1.ObjectMeta{
			Name: fmt.Sprintf("d8:manage:%s:%s", scope, role),
			Labels: map[string]string{
				"heritage":                              "deckhouse",
				"rbac.deckhouse.io/kind":                "manage",
				"rbac.deckhouse.io/aggregate-to-all-as": role,
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []apimachineryv1.LabelSelector{
				{
					MatchLabels: map[string]string{
						"rbac.deckhouse.io/kind":              "manage",
						fmt.Sprintf(scopeTemplate, scope):     "true",
						"rbac.deckhouse.io/aggregate-to-role": role,
					},
				},
			},
		},
	}
	if role == "viewer" {
		cr.ObjectMeta.Labels["rbac.deckhouse.io/aggregate-to-role"] = "manager"
		cr.ObjectMeta.Labels[fmt.Sprintf(scopeTemplate, scope)] = "true"
	}
	return cr
}

func (i *installer) ensureRoles(ctx context.Context, roles []*rbacv1.ClusterRole) *multierror.Error {
	for _, role := range roles {
		i.tasks.Go(func() error {
			return i.ensureRole(ctx, role)
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
		if reflect.DeepEqual(role.Rules, found.Rules) && reflect.DeepEqual(role.GetLabels(), found.GetLabels()) {
			return nil
		}
		_, err = i.client.RbacV1().ClusterRoles().Update(ctx, role, apimachineryv1.UpdateOptions{})
		return err
	})
}
