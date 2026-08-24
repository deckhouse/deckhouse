/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const retiredIstioOperatorRevision = "v1x21"

func isRetiredIstioOperatorRevision(revision string) bool {
	return revision == retiredIstioOperatorRevision
}

// Helper for DKP 1.77 -> 1.78 upgrade. Remove in 1.79.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Run after discovery_versions_to_install validates the configured versions and
	// before discovery_operator_versions_to_install inspects operator resources.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 8},
}, dependency.WithExternalDependencies(cleanupRetiredIstioOperator))

func cleanupRetiredIstioOperator(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	resource := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioNamespace)
	iops, err := resource.List(ctx, metav1.ListOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list IstioOperator resources: %w", err)
	}

	waitingForCleanup := false
	for _, iop := range iops.Items {
		revision, _, err := unstructured.NestedString(iop.Object, "spec", "revision")
		if err != nil {
			return fmt.Errorf("read revision from IstioOperator %q: %w", iop.GetName(), err)
		}
		if !isRetiredIstioOperatorRevision(revision) {
			continue
		}

		waitingForCleanup = true
		if iop.GetDeletionTimestamp() != nil {
			continue
		}

		input.Logger.Info("Deleting retired IstioOperator",
			slog.String("name", iop.GetName()),
			slog.String("namespace", istioNamespace),
			slog.String("revision", revision))

		if err := resource.Delete(ctx, iop.GetName(), metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete retired IstioOperator %q: %w", iop.GetName(), err)
		}
	}

	if waitingForCleanup {
		return fmt.Errorf("waiting for retired Istio revision %s to be cleaned up", retiredIstioOperatorRevision)
	}

	return nil
}
