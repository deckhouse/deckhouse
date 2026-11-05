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
	"path/filepath"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// list of CRDs to delete, like "externalmodulesources.deckhouse.io"
var deprecatedCRDs = []string{}

// licensingCRDs are the CRD files installed only where licensing runs (see
// app.LicensingEnabled): without them the Console has no licensing section.
var licensingCRDs = []string{"clusterlicense.yaml", "effectivelicense.yaml"}

type kubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	InvalidateDiscoveryCache()
}

var defaultLabels = map[string]string{
	crdinstaller.LabelHeritage: "deckhouse",
}

// installCRDFile picks the CRD manifests to install: no translated docs, and no
// licensing CRDs where licensing does not run.
func installCRDFile(crdFilePath string) bool {
	name := filepath.Base(crdFilePath)
	if !app.LicensingEnabled() && slices.Contains(licensingCRDs, name) {
		return false
	}
	return !strings.HasPrefix(name, "doc-")
}

// EnsureCRDs installs or update primary CRDs for deckhouse-controller
func EnsureCRDs(ctx context.Context, client kubeClient) error {
	crds, err := filepath.Glob(app.PathDeckhouseCRDs)
	if err != nil {
		return fmt.Errorf("glob %q: %w", app.PathDeckhouseCRDs, err)
	}

	// The first thing deckhouse-controller does on start, so licensing is settled
	// here once for the CRDs below and for the licensing controller registered later.
	app.ResolveLicensing(ctx, client.Dynamic())

	inst := crdinstaller.NewCRDsInstaller(
		client.Dynamic(),
		crds,
		crdinstaller.WithExtraLabels(defaultLabels),
		crdinstaller.WithFileFilter(installCRDFile),
		crdinstaller.WithMutateFunc(func(crd *unstructured.Unstructured) error {
			if crd.GetName() != "modules.deckhouse.io" || !app.ModuleV2Enabled() {
				return nil
			}

			versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
			for _, v := range versions {
				m := v.(map[string]any)
				m["storage"] = m["name"] == "v1beta1"
				m["served"] = true
			}

			return unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions")
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
