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
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// list of CRDs to delete, like "externalmodulesources.deckhouse.io"
var deprecatedCRDs = []string{
	"moduledocumentations.deckhouse.io",
}

var (
	// moduleDocumentationGVR is the resource of the removed ModuleDocumentation CRD.
	moduleDocumentationGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "moduledocumentations"}
	// moduleDocumentationFinalizer was stripped only by the removed documentation controller.
	moduleDocumentationFinalizer = "modules.deckhouse.io/documentation-exists"
	// documentationLeaseLabel marked the leases docs-builder pods announced themselves with to the
	// removed documentation push; a pod killed without cleanup left its lease behind.
	documentationLeaseLabel = "deckhouse.io/documentation-builder-sync"
)

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

	// DeleteCRDs keeps a CRD that still has objects, and the objects would hang on the finalizer
	if err = deleteModuleDocumentations(ctx, client.Dynamic()); err != nil {
		log.Warn("Couldn't delete module documentations", log.Err(err))
	}
	if err = deleteDocumentationLeases(ctx, client); err != nil {
		log.Warn("Couldn't delete the leases of the documentation push", log.Err(err))
	}

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

// deleteModuleDocumentations strips the documentation finalizer from every ModuleDocumentation and
// deletes it, so the deprecated CRD can go. A cluster without the CRD has nothing to delete.
func deleteModuleDocumentations(ctx context.Context, cli dynamic.Interface) error {
	res := cli.Resource(moduleDocumentationGVR)

	list, err := res.List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("list module documentations: %w", err)
	}

	for _, md := range list.Items {
		if finalizers := md.GetFinalizers(); slices.Contains(finalizers, moduleDocumentationFinalizer) {
			patch, err := json.Marshal(map[string]any{
				"metadata": map[string]any{
					"finalizers": slices.DeleteFunc(finalizers, func(f string) bool { return f == moduleDocumentationFinalizer }),
				},
			})
			if err != nil {
				return fmt.Errorf("marshal finalizers patch: %w", err)
			}

			if _, err = res.Patch(ctx, md.GetName(), types.MergePatchType, patch, metav1.PatchOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("remove the finalizer from the '%s' module documentation: %w", md.GetName(), err)
			}
		}

		if err = res.Delete(ctx, md.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the '%s' module documentation: %w", md.GetName(), err)
		}
	}

	return nil
}

// deleteDocumentationLeases deletes the leases of the removed documentation push: nothing reads them.
func deleteDocumentationLeases(ctx context.Context, cli kubernetes.Interface) error {
	leases := cli.CoordinationV1().Leases(app.NamespaceDeckhouse)

	list, err := leases.List(ctx, metav1.ListOptions{LabelSelector: documentationLeaseLabel})
	if err != nil {
		return fmt.Errorf("list the leases of the documentation push: %w", err)
	}

	for _, lease := range list.Items {
		if err = leases.Delete(ctx, lease.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the '%s' lease: %w", lease.Name, err)
		}
	}

	return nil
}
