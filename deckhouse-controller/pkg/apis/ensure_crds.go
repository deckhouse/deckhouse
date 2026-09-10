// Copyright 2023 Flant JSC
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

package apis

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// moduleCRDFileName is the manifest of the Module CRD, the only one the module
// package sync applies differently.
const moduleCRDFileName = "module.yaml"

// list of CRDs to delete, like "externalmodulesources.deckhouse.io"
var deprecatedCRDs = []string{}

type kubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	InvalidateDiscoveryCache()
}

var defaultLabels = map[string]string{
	crdinstaller.LabelHeritage: "deckhouse",
}

// EnsureCRDs installs or update primary CRDs for deckhouse-controller.
// With the module package sync on, the Module CRD is applied with v1alpha2
// served and stored. The manifest on disk carries v1alpha1 as the stored
// version, so a start without the flag returns the cluster to it.
func EnsureCRDs(ctx context.Context, client kubeClient, crdsGlob string) error {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return fmt.Errorf("glob %q: %w", crdsGlob, err)
	}

	// Replace module CRD with v1alpha2 (served and stored) 
	if app.ModulePackageSyncEnabled() {
		index := slices.IndexFunc(crds, func(path string) bool {
			return filepath.Base(path) == moduleCRDFileName
		})
		if index < 0 {
			return fmt.Errorf("no %s among the crd manifests", moduleCRDFileName)
		}

		rendered, err := moduleCRDServingV1Alpha2(crds[index])
		if err != nil {
			return fmt.Errorf("render the module crd: %w", err)
		}

		defer func() { _ = os.Remove(rendered) }()

		crds[index] = rendered

		log.Info("Module v1alpha2 is served and stored")
	}

	inst := crdinstaller.NewCRDsInstaller(
		client.Dynamic(),
		crds,
		crdinstaller.WithExtraLabels(defaultLabels),
		crdinstaller.WithFileFilter(func(crdFilePath string) bool {
			return !strings.HasPrefix(filepath.Base(crdFilePath), "doc-")
		}),
	)

	deletedCRDs, err := inst.DeleteCRDs(ctx, deprecatedCRDs)
	if err != nil {
		log.Warn("Couldn't delete deprecated CRDs", log.Err(err))
	} else {
		log.Info("The following deprecated CRDs were deleted", slog.String("crds", strings.Join(deletedCRDs, ",")))
	}

	err = inst.Run(ctx)

	// it's not necessary, but it could speed up a bit further api discovery
	client.InvalidateDiscoveryCache()

	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

// moduleCRDServingV1Alpha2 writes a copy of the Module CRD manifest that serves
// every version and stores v1alpha2, and returns the path of the copy. The
// installer reads manifests by path, so the copy is a temp file the caller
// removes once the install is over. The copy differs from the manifest in the
// two flags and nothing else, which is why the document is edited as it was
// read: rebuilding it through the typed CustomResourceDefinition would drop
// every field that type does not carry.
func moduleCRDServingV1Alpha2(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}

	crd := make(map[string]any)
	if err = yaml.Unmarshal(raw, &crd); err != nil {
		return "", fmt.Errorf("unmarshal %q: %w", path, err)
	}

	spec, ok := crd["spec"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%q has no spec", path)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return "", fmt.Errorf("%q has no versions", path)
	}

	for _, item := range versions {
		version, ok := item.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%q has a malformed version", path)
		}

		// v1alpha1 keeps being served for the readers that still ask for it
		version["served"] = true
		version["storage"] = version["name"] == v1alpha2.SchemeGroupVersion.Version
	}

	if raw, err = yaml.Marshal(crd); err != nil {
		return "", fmt.Errorf("marshal %q: %w", path, err)
	}

	file, err := os.CreateTemp("", "module-crd-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create a temp manifest: %w", err)
	}
	defer file.Close()

	if _, err = file.Write(raw); err != nil {
		_ = os.Remove(file.Name())

		return "", fmt.Errorf("write %q: %w", file.Name(), err)
	}

	return file.Name(), nil
}
