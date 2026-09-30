/*
Copyright 2023 Flant JSC

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
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/crd"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
)

const (
	istioNamespace = "d8-istio"
)

type IstioOperatorCrdInfo struct {
	Name     string
	Revision string
}

func applyIstioOperatorFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var iop crd.IstioOperator

	err := sdk.FromUnstructured(obj, &iop)
	if err != nil {
		return nil, err
	}

	return IstioOperatorCrdInfo{
		Name:     iop.GetName(),
		Revision: iop.Spec.Revision,
	}, nil
}

type IstioInfo struct {
	Namespace string
	Revision  string
}

// parseIstio returns the control-plane namespace of an Istio CR and its revision.
// Istio CRs have no revision field, the module names them after their revisions.
func parseIstio(obj *unstructured.Unstructured) (IstioInfo, error) {
	namespace, _, err := unstructured.NestedString(obj.Object, "spec", "namespace")
	if err != nil {
		return IstioInfo{}, err
	}

	return IstioInfo{
		Namespace: namespace,
		Revision:  obj.GetName(),
	}, nil
}

type IstioRevisionInfo struct {
	Namespace string
	Revision  string
}

func applyIstioRevisionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return parseIstioRevision(obj)
}

func parseIstioRevision(obj *unstructured.Unstructured) (IstioRevisionInfo, error) {
	namespace, _, err := unstructured.NestedString(obj.Object, "spec", "namespace")
	if err != nil {
		return IstioRevisionInfo{}, err
	}

	return IstioRevisionInfo{
		Namespace: namespace,
		Revision:  getIstioRevisionRevision(obj),
	}, nil
}

// getIstioRevisionRevision returns the revision of the Istio CR owning the IstioRevision.
// The owner is used rather than the name, as the RevisionBased strategy appends the version
// to the name, e.g. "v1x25-v1-25-2". Without an owner, the name is used.
func getIstioRevisionRevision(obj *unstructured.Unstructured) string {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.APIVersion == istioGVR.GroupVersion().String() && owner.Kind == "Istio" && ptr.Deref(owner.Controller, false) {
			return owner.Name
		}
	}
	return obj.GetName()
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("discovery"),
	// Relies on hook discovery_versions_to_install.go (Order: 5) and must run before hooks deprecated_versions_monitoring.go and compatibility_version_istio_k8s_monitoring.go (Order: 10)
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 9},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Re-runs the module to remove the operator once its last IstioRevision is gone.
			// Status and deletionTimestamp updates don't change the filter result, so they don't trigger the hook.
			Name:                         "istiorevisions",
			ApiVersion:                   "sailoperator.io/v1",
			Kind:                         "IstioRevision",
			FilterFunc:                   applyIstioRevisionFilter,
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
}, dependency.WithExternalDependencies(operatorRevisionsToInstallDiscovery))

func operatorRevisionsToInstallDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	var operatorVersionsToInstall = make([]string, 0)
	var unsupportedRevisions = make([]string, 0)

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())

	var versionsToInstall = input.Values.Get("istio.internal.versionsToInstall").Array()
	for _, versionResult := range versionsToInstall {
		version := versionResult.String()
		if !versionMap.DoesVersionSupportOperator(version) {
			continue
		}
		operatorVersionsToInstall = append(operatorVersionsToInstall, version)
	}

	addRevision := func(revision string) {
		if !versionMap.IsRevisionSupported(revision) {
			if isRetiredIstioOperatorRevision(revision) {
				return
			}
			if !lib.Contains(unsupportedRevisions, revision) {
				unsupportedRevisions = append(unsupportedRevisions, revision)
			}
			return
		}
		version := versionMap.GetVersionByRevision(revision)
		if !versionMap.DoesVersionSupportOperator(version) {
			return
		}
		if !lib.Contains(operatorVersionsToInstall, version) {
			operatorVersionsToInstall = append(operatorVersionsToInstall, version)
		}
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	iops, err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// The CRD can be absent on operator-free control planes; this is expected.
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		for _, iop := range iops.Items {
			infoAny, err := applyIstioOperatorFilter(&iop)
			if err != nil {
				return fmt.Errorf("cannot parse IstioOperator %q: %w", iop.GetName(), err)
			}
			iopInfo, ok := infoAny.(IstioOperatorCrdInfo)
			if !ok {
				return fmt.Errorf("unexpected IstioOperator filter result type for %q", iop.GetName())
			}
			addRevision(iopInfo.Revision)
		}
	}

	istios, err := k8sClient.Dynamic().Resource(istioGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// The CRD can be absent on old control planes; this is expected.
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		for _, istio := range istios.Items {
			istioInfo, err := parseIstio(&istio)
			if err != nil {
				return fmt.Errorf("cannot parse Istio %q: %w", istio.GetName(), err)
			}
			// Istio CRs are cluster-scoped, skip the ones of foreign control-planes.
			if istioInfo.Namespace != istioNamespace {
				continue
			}
			addRevision(istioInfo.Revision)
		}
	}

	// The Istio CR has no finalizer and is gone right after helm deletes it, while its IstioRevision
	// waits for the operator to uninstall istiod. Keep the operator until all its IstioRevisions are gone.
	istioRevisions, err := sdkobjectpatch.UnmarshalToStruct[IstioRevisionInfo](input.Snapshots, "istiorevisions")
	if err != nil {
		return fmt.Errorf("failed to unmarshal istiorevisions snapshot: %w", err)
	}
	for _, istioRevision := range istioRevisions {
		// IstioRevisions are cluster-scoped, skip the ones of foreign control-planes.
		if istioRevision.Namespace != istioNamespace {
			continue
		}
		addRevision(istioRevision.Revision)
	}

	if len(unsupportedRevisions) > 0 {
		sort.Strings(unsupportedRevisions)
		return fmt.Errorf("unsupported revisions: [%s]", strings.Join(unsupportedRevisions, ","))
	}

	sort.Strings(operatorVersionsToInstall)
	input.Values.Set("istio.internal.operatorVersionsToInstall", operatorVersionsToInstall)

	return nil
}
