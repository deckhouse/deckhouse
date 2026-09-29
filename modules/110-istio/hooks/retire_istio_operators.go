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

// An IstioOperator of a version that is no longer installed must be deleted
// without waiting. Its istio-operator does not remove the control plane while
// any proxy is still connected to it, and those proxies only move once the
// release installs the new global version. But nelm, which deploys the module
// release, deletes resources that left the chart before it installs anything
// else, and waits for each of them to be gone. That wait cannot be turned off,
// and for this IstioOperator it never ends.
//
// So this hook marks such an IstioOperator with helm.sh/resource-policy=keep,
// which makes nelm leave it alone, and deletes it in the background. The
// istio-operator keeps running while the IstioOperator exists (see
// discovery_operator_versions_to_install.go) and removes it once no proxy is
// connected.

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
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
)

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"
)

var keepResourcePatch = map[string]interface{}{
	"metadata": map[string]interface{}{
		"annotations": map[string]interface{}{
			helmResourcePolicyAnnotation: helmResourcePolicyKeep,
		},
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Relies on versionsToInstall from discovery_versions_to_install.go (Order: 5), and
	// the patches must be applied before nelm deploys the release.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 6},
}, dependency.WithExternalDependencies(retireIstioOperators))

func retireIstioOperators(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())

	versionsToInstallResult := input.Values.Get("istio.internal.versionsToInstall").Array()
	versionsToInstall := make([]string, 0, len(versionsToInstallResult))
	for _, versionResult := range versionsToInstallResult {
		versionsToInstall = append(versionsToInstall, versionResult.String())
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	iops, err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// The CRD can be absent on operator-free control planes; this is expected.
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("list IstioOperator resources: %w", err)
	}

	for _, iop := range iops.Items {
		revision, _, err := unstructured.NestedString(iop.Object, "spec", "revision")
		if err != nil {
			return fmt.Errorf("read revision from IstioOperator %q: %w", iop.GetName(), err)
		}

		// An unknown revision is reported by discovery_operator_versions_to_install.go.
		version := versionMap.GetVersionByRevision(revision)
		if version == "" || lib.Contains(versionsToInstall, version) {
			continue
		}

		// Also for an IstioOperator that is already being deleted: nelm may
		// have deleted it before this hook existed, and would wait for it again.
		if iop.GetAnnotations()[helmResourcePolicyAnnotation] != helmResourcePolicyKeep {
			input.Logger.Info("Keeping IstioOperator of a version no longer installed out of the release",
				slog.String("name", iop.GetName()), slog.String("revision", revision))
			input.PatchCollector.PatchWithMerge(keepResourcePatch, iopGVR.GroupVersion().String(), "IstioOperator", istioNamespace, iop.GetName())
		}

		if iop.GetDeletionTimestamp() == nil {
			input.Logger.Info("Deleting IstioOperator of a version no longer installed",
				slog.String("name", iop.GetName()), slog.String("revision", revision))
			input.PatchCollector.DeleteInBackground(iopGVR.GroupVersion().String(), "IstioOperator", istioNamespace, iop.GetName())
		}
	}

	return nil
}
