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
	"strings"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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

	cp, err := NewCRDsInstaller(client.Dynamic(), crdsGlob)
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	return cp.Run(context.TODO())
}

// CRDsInstaller simultaneously installs CRDs from specified directory
type CRDsInstaller = addonoperator.CRDsInstaller

// NewCRDsInstaller creates new installer for CRDs
// crdsGlob example: "/deckhouse/modules/002-deckhouse/crds/*.yaml"
func NewCRDsInstaller(client dynamic.Interface, crdsGlob string) (*CRDsInstaller, error) {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", crdsGlob, err)
	}

	return addonoperator.NewCRDsInstaller(
		client,
		crds,
		addonoperator.WithExtraLabels(defaultLabels),
		addonoperator.WithFileFilter(func(crdFilePath string) bool {
			return !strings.HasPrefix(filepath.Base(crdFilePath), "doc-")
		}),
	), nil
}
