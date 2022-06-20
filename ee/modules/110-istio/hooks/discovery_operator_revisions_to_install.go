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
	var operatorRevisionsToInstall = make([]string, 0)

	var supportedRevisions []string
	var supportedVersionsResult = input.Values.Get("istio.internal.supportedVersions").Array()
	for _, versionResult := range supportedVersionsResult {
		supportedRevisions = append(supportedRevisions, internal.VersionToRevision(versionResult.String()))
	}

	var revisionsToInstallResult = input.Values.Get("istio.internal.revisionsToInstall").Array()
	for _, revisionResult := range revisionsToInstallResult {
		operatorRevisionsToInstall = append(operatorRevisionsToInstall, revisionResult.String())
	}

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

	sort.Strings(operatorRevisionsToInstall)
	input.Values.Set("istio.internal.operatorRevisionsToInstall", operatorRevisionsToInstall)

	return nil
}
