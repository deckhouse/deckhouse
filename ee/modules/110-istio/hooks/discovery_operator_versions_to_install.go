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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/crd"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/istio_versions"
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

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istiooperators",
			ApiVersion:        "install.istio.io/v1alpha1",
			Kind:              "IstioOperator",
			FilterFunc:        applyIstioOperatorFilter,
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, operatorRevisionsToInstallDiscovery)

func operatorRevisionsToInstallDiscovery(input *go_hook.HookInput) error {
	var operatorVersionsToInstall = make([]string, 0)
	var unsupportedRevisions = make([]string, 0)

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())

	var versionsToInstallResult = input.Values.Get("istio.internal.versionsToInstall").Array()
	for _, versionResult := range versionsToInstallResult {
		operatorVersionsToInstall = append(operatorVersionsToInstall, versionResult.String())
	}

	for _, iop := range input.Snapshots["istiooperators"] {
		iopInfo := iop.(IstioOperatorCrdInfo)
		iopVer := versionMap.GetVersionByRevision(iopInfo.Revision)
		if !versionMap.IsRevisionSupported(iopInfo.Revision) {
			unsupportedRevisions = append(unsupportedRevisions, iopInfo.Revision)
			continue
		}
		if !internal.Contains(operatorVersionsToInstall, iopVer) {
			operatorVersionsToInstall = append(operatorVersionsToInstall, iopVer)
		}
	}

	if len(unsupportedRevisions) > 0 {
		sort.Strings(unsupportedRevisions)
		return fmt.Errorf("unsupported revisions: [%s]", strings.Join(unsupportedRevisions, ","))
	}

	sort.Strings(operatorVersionsToInstall)
	input.Values.Set("istio.internal.operatorVersionsToInstall", operatorVersionsToInstall)

	return nil
}
