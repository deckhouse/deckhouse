/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// The Order below matters for ensure_crds_istio.go, it needs globalVersion to deploy proper CRDs
	OnStartup:    &go_hook.OrderedConfig{Order: 5},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(revisionsDiscovery))

func revisionsDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	var globalRevision string
	var revisionsToInstall = make([]string, 0)
	var unsupportedVersions []string

	var supportedVersions []string
	var supportedVersionsResult = input.Values.Get("istio.internal.supportedVersions").Array()
	for _, versionResult := range supportedVersionsResult {
		supportedVersions = append(supportedVersions, versionResult.String())
	}

	var globalVersion string

	switch {
	case input.ConfigValues.Exists("istio.globalVersion"):
		// globalVersion is set in CM — use it
		globalVersion = input.ConfigValues.Get("istio.globalVersion").String()

		if !internal.Contains(supportedVersions, globalVersion) {
			unsupportedVersions = append(unsupportedVersions, globalVersion)
		}
	case input.Values.Exists("istio.internal.globalVersion"):
		// globalVersion was previously discovered — use it
		globalVersion = input.Values.Get("istio.internal.globalVersion").String()
	default:
		// maybe there is a global istiod Service with annotation?
		k8sClient, err := dc.GetK8sClient()
		if err != nil {
			return err
		}

		service, err := k8sClient.CoreV1().Services("d8-istio").Get(context.TODO(), "istiod", metav1.GetOptions{})
		if err == nil {
			// there is the global istiod Service — let's get the annotation
			if version, ok := service.GetAnnotations()["istio.deckhouse.io/global-version"]; ok {
				globalVersion = version
			} else {
				return fmt.Errorf("can't find istio.deckhouse.io/global-version annotation for istiod global Service d8-istio/istiod")
			}
		}
	}

	// couldn't discover globalVersion — let's use default value from openapi/config-values.yaml
	if globalVersion == "" {
		globalVersion = input.Values.Get("istio.globalVersion").String()
	}

	globalRevision = internal.VersionToRevision(globalVersion)

	var additionalRevisions []string
	var additionalVersionsResult = input.ConfigValues.Get("istio.additionalVersions").Array()
	for _, versionResult := range additionalVersionsResult {
		rev := internal.VersionToRevision(versionResult.String())
		if !internal.Contains(additionalRevisions, rev) {
			additionalRevisions = append(additionalRevisions, rev)
			if !internal.Contains(supportedVersions, versionResult.String()) {
				unsupportedVersions = append(unsupportedVersions, versionResult.String())
			}
		}
	}

	revisionsToInstall = append(revisionsToInstall, additionalRevisions...)
	if !internal.Contains(revisionsToInstall, globalRevision) {
		revisionsToInstall = append(revisionsToInstall, globalRevision)
	}

	if len(unsupportedVersions) > 0 {
		sort.Strings(unsupportedVersions)
		return fmt.Errorf("unsupported versions: [%s]", strings.Join(unsupportedVersions, ","))
	}

	sort.Strings(revisionsToInstall)

	input.Values.Set("istio.internal.globalVersion", globalVersion)
	input.Values.Set("istio.internal.globalRevision", globalRevision)
	input.Values.Set("istio.internal.revisionsToInstall", revisionsToInstall)

	return nil
}
