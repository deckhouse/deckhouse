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
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
)

const (
	moduleValuesStoreSecretName      = "module-values-store"
	moduleValuesStoreSecretNamespace = "d8-istio"
	maxIstioVersionBeenInClusterKey  = "maxIstioVersionBeenInCluster"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup:    &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_versions_to_install.go
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_versions_to_install.go
}, dependency.WithExternalDependencies(ensureCRDs))

func ensureCRDs(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	// collect all istio versions (global + additional | uniq)
	istioVersions := make([]string, 0)

	if !input.Values.Get("istio.internal.globalVersion").Exists() {
		return fmt.Errorf("istio.internal.globalVersion is not discovered by discovery_versions_to_install.go yet")
	}
	globalVersion := input.Values.Get("istio.internal.globalVersion").String()
	istioVersions = append(istioVersions, globalVersion)

	for _, versionResult := range input.ConfigValues.Get("istio.additionalVersions").Array() {
		if !lib.Contains(istioVersions, versionResult.String()) {
			istioVersions = append(istioVersions, versionResult.String())
		}
	}

	// semvers is a slice for sorting by semver
	semvers := make([]*semver.Version, len(istioVersions))
	for i, version := range istioVersions {
		v, err := semver.NewVersion(version)
		if err != nil {
			return err
		}
		semvers[i] = v
	}

	sort.Sort(semver.Collection(semvers))

	CRDversionToInstall, err := resolveCRDBundleVersion(ctx, input, dc, semvers[len(semvers)-1])
	if err != nil {
		return err
	}

	crdsGlob := "/deckhouse/modules/110-istio/_crds/istio/" + CRDversionToInstall + "/*.yaml"

	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return fmt.Errorf("invalid glob pattern: %w", err)
	}
	if len(crds) == 0 {
		return fmt.Errorf("no CRD files found matching pattern: %s", crdsGlob)
	}

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())
	if !versionMap.DoesVersionSupportOperator(CRDversionToInstall) {
		for _, crdFile := range crds {
			fileName := filepath.Base(crdFile)
			if fileName == "crd-operator.yaml" || strings.HasPrefix(fileName, "sailoperator.io_") {
				return fmt.Errorf("unsupported CRD file for operator-free version %s: %s", CRDversionToInstall, fileName)
			}
		}
	}

	return ensure_crds.EnsureCRDsHandler(crdsGlob)(ctx, input, dc)
}

func resolveCRDBundleVersion(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, desiredVersion *semver.Version) (string, error) {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return "", err
	}

	versionToInstall := desiredVersion

	secret, err := k8sClient.CoreV1().Secrets(moduleValuesStoreSecretNamespace).Get(ctx, moduleValuesStoreSecretName, metav1.GetOptions{})
	switch {
	case err == nil:
		storedMaxVersion := string(secret.Data[maxIstioVersionBeenInClusterKey])
		if storedMaxVersion != "" {
			stored, parseErr := semver.NewVersion(storedMaxVersion)
			if parseErr != nil {
				return "", fmt.Errorf("invalid %s value %q in secret %s/%s: %w",
					maxIstioVersionBeenInClusterKey, storedMaxVersion, moduleValuesStoreSecretNamespace, moduleValuesStoreSecretName, parseErr)
			}
			if stored.GreaterThan(versionToInstall) {
				versionToInstall = stored
			}
		}
	case apierrors.IsNotFound(err):
		// no secret yet
	default:
		return "", fmt.Errorf("failed to get secret %s/%s: %w", moduleValuesStoreSecretNamespace, moduleValuesStoreSecretName, err)
	}

	CRDversionToInstall := fmt.Sprintf("%d.%d", versionToInstall.Major(), versionToInstall.Minor())

	if err := upsertMaxIstioVersionBeenInCluster(ctx, input, k8sClient, CRDversionToInstall); err != nil {
		return "", err
	}

	return CRDversionToInstall, nil
}

func upsertMaxIstioVersionBeenInCluster(ctx context.Context, input *go_hook.HookInput, k8sClient k8s.Client, version string) error {
	_, err := k8sClient.CoreV1().Namespaces().Get(ctx, moduleValuesStoreSecretNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get namespace %s: %w", moduleValuesStoreSecretNamespace, err)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      moduleValuesStoreSecretName,
			Namespace: moduleValuesStoreSecretNamespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "istio",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			maxIstioVersionBeenInClusterKey: []byte(version),
		},
	}

	input.PatchCollector.CreateOrUpdate(secret)
	return nil
}
