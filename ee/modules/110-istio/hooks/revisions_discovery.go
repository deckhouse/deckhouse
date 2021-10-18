/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/private"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/private/crd"
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
	Revision string
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespace v1.Namespace
	err := sdk.FromUnstructured(obj, &namespace)
	if err != nil {
		return "", err
	}

	var namespaceInfo = NamespaceInfo{
		Name:     namespace.GetName(),
		Revision: "",
	}

	if revision, ok := namespace.Labels["istio.io/rev"]; ok {
		namespaceInfo.Revision = revision
	}

	return namespaceInfo, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: private.Queue("revisions-discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istiooperators",
			ApiVersion:        "install.istio.io/v1alpha1",
			Kind:              "IstioOperator",
			FilterFunc:        applyIstioOperatorFilter,
			NamespaceSelector: private.NsSelector(),
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
	},
	Schedule: []go_hook.ScheduleConfig{ // Due to we are afraid of subscribing to all Pods in the cluster,
		{Name: "cron", Crontab: "0 * * * *"}, // we run the hook every hour to discover unused revisions.
	},
}, dependency.WithExternalDependencies(revisionsDiscovery))

func versionToRevision(version string) string {
	version = "v" + version

	// v1.2.3-alpha.4 -> v1.2.3-alpha4
	var re = regexp.MustCompile(`([a-z])\.([0-9])`)
	version = re.ReplaceAllString(version, `$1$2`)

	// v1.2.3-alpha4 -> v1x2x3-alpha4
	version = strings.ReplaceAll(version, ".", "x")

	// v1x2x3-alpha4 -> v1x2x3alpha4
	version = strings.ReplaceAll(version, "-", "")

	return version
}

func revisionsDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	var globalRevision string
	var revisionsToInstall = make([]string, 0)
	var operatorRevisionsToInstall = make([]string, 0)
	var applicationNamespaces = make([]string, 0)

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	var supportedRevisions []string

	var supportedVersionsResult = input.Values.Get("istio.internal.supportedVersions").Array()
	for _, versionResult := range supportedVersionsResult {
		supportedRevisions = append(supportedRevisions, versionToRevision(versionResult.String()))
	}

	var globalVersion string
	if input.Values.Exists("istio.globalVersion") {
		globalVersion = input.Values.Get("istio.globalVersion").String()
	} else {
		globalVersion = supportedVersionsResult[len(supportedVersionsResult)-1].String()
	}
	globalRevision = versionToRevision(globalVersion)

	revisionsToInstall = append(revisionsToInstall, globalRevision)
	for _, ns := range input.Snapshots["namespaces_definite_revision"] {
		nsInfo := ns.(NamespaceInfo)
		if !private.Contains(revisionsToInstall, nsInfo.Revision) {
			revisionsToInstall = append(revisionsToInstall, nsInfo.Revision)
		}

		if !private.Contains(applicationNamespaces, nsInfo.Name) {
			applicationNamespaces = append(applicationNamespaces, nsInfo.Name)
		}
	}
	for _, ns := range input.Snapshots["namespaces_global_revision"] {
		nsInfo := ns.(NamespaceInfo)
		if !private.Contains(applicationNamespaces, nsInfo.Name) {
			applicationNamespaces = append(applicationNamespaces, nsInfo.Name)
		}
	}

	podList, err := k8sClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "istio.io/rev"})
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		rev := pod.Labels["istio.io/rev"]
		ns := pod.GetNamespace()

		if !private.Contains(revisionsToInstall, rev) {
			revisionsToInstall = append(revisionsToInstall, rev)
		}
		if !private.Contains(applicationNamespaces, ns) {
			applicationNamespaces = append(applicationNamespaces, ns)
		}
	}

	operatorRevisionsToInstall = append(operatorRevisionsToInstall, revisionsToInstall...)
	for _, iop := range input.Snapshots["istiooperators"] {
		iopInfo := iop.(IstioOperatorCrdInfo)
		if !private.Contains(operatorRevisionsToInstall, iopInfo.Revision) {
			operatorRevisionsToInstall = append(operatorRevisionsToInstall, iopInfo.Revision)
		}
	}

	var unsupportedRevisions []string
	for _, rev := range operatorRevisionsToInstall {
		if !private.Contains(supportedRevisions, rev) {
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

	return nil
}
