/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
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

type NamespaceInfo struct {
	Name     string
	Revision string // for revisions_monitoring.go
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespace v1.Namespace
	err := sdk.FromUnstructured(obj, &namespace)
	if err != nil {
		return "", err
	}

	var namespaceInfo = NamespaceInfo{
		Name: namespace.GetName(),
	}

	if revision, ok := namespace.Labels["istio.io/rev"]; ok {
		namespaceInfo.Revision = revision
	} else {
		namespaceInfo.Revision = "global"
	}

	return namespaceInfo, nil
}

type GlobalServiceInfo struct {
	Version string
}

func applyGlobalServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service
	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return "", err
	}

	var globalServiceInfo = GlobalServiceInfo{
		Version: "",
	}

	if version, ok := service.GetAnnotations()["istio.deckhouse.io/global-version"]; ok {
		globalServiceInfo.Version = version
	} else {
		// migration from v1.10.1: delete this "else" after deploying to all clusters
		globalServiceInfo.Version = "1.10.1"
	}

	return globalServiceInfo, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("revisions-discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istiooperators",
			ApiVersion:        "install.istio.io/v1alpha1",
			Kind:              "IstioOperator",
			FilterFunc:        applyIstioOperatorFilter,
			NamespaceSelector: internal.NsSelector(),
		},
		{
			Name:          "namespaces_global_revision",
			ApiVersion:    "v1",
			Kind:          "Namespace",
			FilterFunc:    applyNamespaceFilter,
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"istio-injection": "enabled"}},
		},
		{
			Name:       "namespaces_definite_revision",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: "Exists",
					},
				},
			},
		},
		{
			Name:              "service_global_revision",
			ApiVersion:        "v1",
			Kind:              "Service",
			FilterFunc:        applyGlobalServiceFilter,
			NamespaceSelector: internal.NsSelector(),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"istiod"},
			},
		},
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5}, // Sequence matters for ensure_crds_istio.go
	OnStartup:    &go_hook.OrderedConfig{Order: 5}, // Sequence matters for ensure_crds_istio.go
}, dependency.WithExternalDependencies(revisionsDiscovery))

func revisionsDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	var globalRevision string
	var revisionsToInstall = make([]string, 0)
	var operatorRevisionsToInstall = make([]string, 0)
	var applicationNamespaces = make([]string, 0)

	var supportedRevisions []string
	var supportedVersionsResult = input.Values.Get("istio.internal.supportedVersions").Array()
	for _, versionResult := range supportedVersionsResult {
		supportedRevisions = append(supportedRevisions, internal.VersionToRevision(versionResult.String()))
	}

	var globalVersion string
	switch {
	case input.ConfigValues.Exists("istio.globalVersion"):
		globalVersion = input.ConfigValues.Get("istio.globalVersion").String()
	case len(input.Snapshots["service_global_revision"]) == 1:
		globalServiceInfo := input.Snapshots["service_global_revision"][0].(GlobalServiceInfo)
		globalVersion = globalServiceInfo.Version
	default:
		globalVersion = input.Values.Get("istio.globalVersion").String() // default from openapi/values.yaml
	}
	globalRevision = internal.VersionToRevision(globalVersion)

	var additionalRevisions []string
	var additionalVersionsResult = input.ConfigValues.Get("istio.additionalVersions").Array()
	for _, versionResult := range additionalVersionsResult {
		rev := internal.VersionToRevision(versionResult.String())
		if !internal.Contains(additionalRevisions, rev) {
			additionalRevisions = append(additionalRevisions, rev)
		}
	}

	revisionsToInstall = append(revisionsToInstall, additionalRevisions...)
	if !internal.Contains(revisionsToInstall, globalRevision) {
		revisionsToInstall = append(revisionsToInstall, globalRevision)
	}

	for _, ns := range append(input.Snapshots["namespaces_definite_revision"], input.Snapshots["namespaces_global_revision"]...) {
		nsInfo := ns.(NamespaceInfo)
		if !internal.Contains(applicationNamespaces, nsInfo.Name) {
			applicationNamespaces = append(applicationNamespaces, nsInfo.Name)
		}
	}

	operatorRevisionsToInstall = append(operatorRevisionsToInstall, revisionsToInstall...)
	for _, iop := range input.Snapshots["istiooperators"] {
		iopInfo := iop.(IstioOperatorCrdInfo)
		if !internal.Contains(operatorRevisionsToInstall, iopInfo.Revision) {
			operatorRevisionsToInstall = append(operatorRevisionsToInstall, iopInfo.Revision)
		}
	}

	var unsupportedRevisions []string
	for _, rev := range operatorRevisionsToInstall {
		if !internal.Contains(supportedRevisions, rev) {
			unsupportedRevisions = append(unsupportedRevisions, rev)
		}
	}
	if len(unsupportedRevisions) > 0 {
		sort.Strings(unsupportedRevisions)
		return fmt.Errorf("unsupported revisions: [%s]", strings.Join(unsupportedRevisions, ","))
	}

	sort.Strings(revisionsToInstall)
	sort.Strings(operatorRevisionsToInstall)
	sort.Strings(applicationNamespaces)

	input.Values.Set("istio.internal.globalRevision", globalRevision)
	input.Values.Set("istio.internal.revisionsToInstall", revisionsToInstall)
	input.Values.Set("istio.internal.operatorRevisionsToInstall", operatorRevisionsToInstall)
	input.Values.Set("istio.internal.applicationNamespaces", applicationNamespaces)
	input.ConfigValues.Set("istio.globalVersion", globalVersion)

	return nil
}
