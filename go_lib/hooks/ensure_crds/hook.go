/*
Copyright 2022 Flant JSC

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

package ensure_crds

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

var defaultLabels = map[string]string{
	"heritage": "deckhouse",
}

func RegisterEnsureCRDsHook(crdsGlob string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnStartup: &go_hook.OrderedConfig{Order: 5},
	}, dependency.WithExternalDependencies(EnsureCRDsHandler(crdsGlob)))
}

func EnsureCRDsHandler(crdsGlob string) func(input *go_hook.HookInput, dc dependency.Container) error {
	return func(input *go_hook.HookInput, dc dependency.Container) error {
		result := EnsureCRDs(crdsGlob, dc)

		if result.ErrorOrNil() != nil {
			input.Logger.Error("ensure_crds failed", slog.String("error", result.Error()))
		}

		return result.ErrorOrNil()
	}
}

func EnsureCRDs(crdsGlob string, dc dependency.Container) *multierror.Error {
	result := new(multierror.Error)

	client, err := dc.GetK8sClient()
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	cp, err := NewCRDsInstaller(client, crdsGlob)
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	return cp.Run(context.TODO())
}

// CRDsInstaller simultaneously installs CRDs from specified directory
type CRDsInstaller struct {
	k8sClient    k8s.Client
	crdFilesPath []string
	installer    *addonoperator.CRDsInstaller

	// concurrent tasks to create resource in a k8s cluster
	k8sTasks *multierror.Group
}

func (cp *CRDsInstaller) DeleteCRDs(ctx context.Context, crdsToDelete []string) ([]string, error) {
	var deletedCRDs []string
	// delete crds listed in crdsToDelete if there are no related custom resources in the cluster
	for _, crdName := range crdsToDelete {
		deleteCRD := true
		crd, err := cp.getCRDFromCluster(ctx, crdName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("error occurred during %s CRD clean up: %w", crdName, err)
			}
			continue
		}

		for _, version := range crd.Spec.Versions {
			gvr := schema.GroupVersionResource{
				Group:    crd.Spec.Group,
				Version:  version.Name,
				Resource: crd.Spec.Names.Plural,
			}
			list, err := cp.k8sClient.Dynamic().Resource(gvr).List(ctx, apimachineryv1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("error occurred listing %s CRD objects of version %s: %w", crdName, version.Name, err)
			}
			if len(list.Items) > 0 {
				deleteCRD = false
				break
			}
		}

		if deleteCRD {
			err := cp.k8sClient.Dynamic().Resource(crdGVR).Delete(ctx, crdName, apimachineryv1.DeleteOptions{})
			if err != nil {
				return nil, fmt.Errorf("error occurred deleting %s CRD: %w", crdName, err)
			}
			deletedCRDs = append(deletedCRDs, crdName)
		}
	}
	return deletedCRDs, nil
}

func (cp *CRDsInstaller) Run(ctx context.Context) *multierror.Error {
	return cp.installer.Run(ctx)
}

func (cp *CRDsInstaller) updateOrInsertCRD(ctx context.Context, crd *v1.CustomResourceDefinition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existCRD, err := cp.getCRDFromCluster(ctx, crd.GetName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				ucrd, err := sdk.ToUnstructured(crd)
				if err != nil {
					return err
				}

				_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Create(ctx, ucrd, apimachineryv1.CreateOptions{})
				return err
			}

			return err
		}

		if existCRD.Spec.Conversion != nil {
			crd.Spec.Conversion = existCRD.Spec.Conversion
		}

		if existCRD.GetObjectMeta().GetLabels()["heritage"] == "deckhouse" &&
			reflect.DeepEqual(existCRD.Spec, crd.Spec) {
			return nil
		}

		existCRD.Spec = crd.Spec
		if len(existCRD.ObjectMeta.Labels) == 0 {
			existCRD.ObjectMeta.Labels = make(map[string]string, 1)
		}
		existCRD.ObjectMeta.Labels["heritage"] = "deckhouse"

		ucrd, err := sdk.ToUnstructured(existCRD)
		if err != nil {
			return err
		}

		_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Update(ctx, ucrd, apimachineryv1.UpdateOptions{})
		return err
	})
}

func (cp *CRDsInstaller) getCRDFromCluster(ctx context.Context, crdName string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}

	o, err := cp.k8sClient.Dynamic().Resource(crdGVR).Get(ctx, crdName, apimachineryv1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = sdk.FromUnstructured(o, &crd)
	if err != nil {
		return nil, err
	}

	return crd, nil
}

// NewCRDsInstaller creates new installer for CRDs
// crdsGlob example: "/deckhouse/modules/002-deckhouse/crds/*.yaml"
func NewCRDsInstaller(client k8s.Client, crdsGlob string) (*CRDsInstaller, error) {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", crdsGlob, err)
	}

	return &CRDsInstaller{
		k8sClient: client,
		installer: addonoperator.NewCRDsInstaller(
			client.Dynamic(),
			crds,
			addonoperator.WithExtraLabels(defaultLabels),
			addonoperator.WithFileFilter(func(crdFilePath string) bool {
				return !strings.HasPrefix(filepath.Base(crdFilePath), "doc-")
			}),
		),
		crdFilesPath: crds,
		k8sTasks:     new(multierror.Group),
	}, nil
}
