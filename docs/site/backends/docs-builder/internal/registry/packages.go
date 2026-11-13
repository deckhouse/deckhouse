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

package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// The modules of package repositories, as deckhouse-controller with the v2
// module manager keeps them: a ModulePackage per module, found in the
// PackageRepositories it lists. Their registries keep the release images of
// a module at <repo>/<module>/release, as a ModuleSource does.
//
// ponytail: a package that publishes its versions only under <module>/version
// and its channels under <module>/release-channel is not found; read those
// paths when such packages appear.
var (
	modulePackageGVR     = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modulepackages"}
	packageRepositoryGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "packagerepositories"}
)

// packageModules lists the modules of the package repositories. A cluster
// without them, or without the resource, has none.
func (s *Sources) packageModules(ctx context.Context) ([]Module, error) {
	list, err := s.client.Resource(modulePackageGVR).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list module packages: %w", err)
	}

	modules := make([]Module, 0, len(list.Items))
	for i := range list.Items {
		obj := &list.Items[i]
		if len(repositoryNames(obj)) == 0 {
			continue
		}
		// The package an installed module runs.
		used, _, _ := unstructured.NestedString(obj.Object, "status", "usedBy", "repositoryName")
		version, _, _ := unstructured.NestedString(obj.Object, "status", "usedBy", "version")
		modules = append(modules, Module{Name: obj.GetName(), Enabled: used != "", Version: version})
	}

	return modules, nil
}

// repositoryNames are the package repositories of a ModulePackage in the
// order to try them: the one the installed module came from first.
func repositoryNames(pkg *unstructured.Unstructured) []string {
	used, _, _ := unstructured.NestedString(pkg.Object, "status", "usedBy", "repositoryName")
	available, _, _ := unstructured.NestedStringSlice(pkg.Object, "status", "availableRepositories")

	names := make([]string, 0, len(available)+1)
	for _, name := range append([]string{used}, available...) {
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return names
}

// packageSources returns the registries of a module of the package
// repositories; with none the error wraps ErrNoSource.
func (s *Sources) packageSources(ctx context.Context, module string) ([]Source, error) {
	pkg, err := s.client.Resource(modulePackageGVR).Get(ctx, module, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("module %s: %w", module, ErrNoSource)
	}
	if err != nil {
		return nil, fmt.Errorf("get module package %s: %w", module, err)
	}

	var sources []Source
	var unanswered []error
	for _, name := range repositoryNames(pkg) {
		src, err := s.packageSource(ctx, name)
		if err == nil {
			sources = append(sources, src)
			continue
		}
		s.logger.Warn("skip package repository", slog.String("module", module), slog.String("package_repository", name), log.Err(err))
		if !apierrors.IsNotFound(err) && !errors.Is(err, errBadSource) {
			unanswered = append(unanswered, err)
		}
	}

	if len(sources) == 0 {
		if len(unanswered) > 0 {
			return nil, fmt.Errorf("module %s: %w", module, errors.Join(unanswered...))
		}
		return nil, fmt.Errorf("module %s: %w", module, ErrNoSource)
	}

	return sources, nil
}

// packageSource is source for a PackageRepository.
func (s *Sources) packageSource(ctx context.Context, name string) (Source, error) {
	repo, err := s.client.Resource(packageRepositoryGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Source{}, err
	}

	src, err := packageSourceFrom(repo)
	if err != nil {
		return Source{}, err
	}

	src, err = s.reachable(ctx, src)
	if err != nil {
		return Source{}, fmt.Errorf("package repository %s: %w", name, err)
	}

	return src, nil
}

// packageSourceFrom is sourceFrom for a PackageRepository, whose registry
// may give a login and a password besides a docker config.
func packageSourceFrom(repo *unstructured.Unstructured) (Source, error) {
	src, err := sourceFrom(repo)
	if err != nil {
		return Source{}, fmt.Errorf("package repository %s: %w: %w", repo.GetName(), errBadSource, err)
	}
	src.Login, _, _ = unstructured.NestedString(repo.Object, "spec", "registry", "login")
	src.Password, _, _ = unstructured.NestedString(repo.Object, "spec", "registry", "password")

	return src, nil
}

// dockerConfig is the base64-encoded docker config that gives host the
// credentials auth, a base64-encoded "login:password".
func dockerConfig(host, auth string) (string, error) {
	cfg, err := json.Marshal(map[string]any{"auths": map[string]any{host: map[string]string{"auth": auth}}})
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(cfg), nil
}
