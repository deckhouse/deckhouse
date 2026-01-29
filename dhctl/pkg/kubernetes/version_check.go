// Copyright 2026 Flant JSC
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

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	gcr "github.com/google/go-containerregistry/pkg/name"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var ErrDeckhouseVersionNotFound = errors.New("deckhouse version not found in cluster")

type VersionCheckOptions struct {
	AllowAnyError         bool
	SkipDhctlVersionCheck bool
}

// DefaultVersionCheckOptions returns VersionCheckOptions with values from app flags.
func DefaultVersionCheckOptions() VersionCheckOptions {
	return VersionCheckOptions{
		SkipDhctlVersionCheck: app.SkipDhctlVersionCheck,
	}
}

func CheckDeckhouseVersionCompatibility(ctx context.Context, kubeCl *client.KubernetesClient, opts VersionCheckOptions) error {
	if app.AppVersion == "local" || app.AppVersion == "dev" {
		log.InfoLn("Deckhouse version check is skipped for local/dev builds")
		return nil
	}
	if opts.SkipDhctlVersionCheck {
		log.WarnF("⚠️  WARNING: Deckhouse version check is skipped manually. This may lead to cluster instability, data corruption, or complete cluster failure. We do NOT guarantee cluster operability when version check is skipped!")
		return nil
	}

	deckhouseVersion, err := getDeckhouseVersionFromCluster(ctx, kubeCl)
	if err != nil {
		if opts.AllowAnyError {
			log.WarnF("Deckhouse version check is skipped: %v\n", err)
			return nil
		}
		return fmt.Errorf("Deckhouse and dhctl version check failed: %w. You may use \"--skip-dhctl-version-check\" flag to skip this check. ⚠️  WARNING: Using \"--skip-dhctl-version-check\" flag may lead to cluster instability, data corruption, or complete cluster failure. We do NOT guarantee cluster operability when version check is skipped!", err)
	}

	if !versionsMatch(app.AppVersion, deckhouseVersion) {
		return fmt.Errorf("Deckhouse and dhctl version mismatch: dhctl=%s, cluster=%s. Use dhctl with tag \"%s\" or use \"--skip-dhctl-version-check\" flag to skip this check. WARNING: Using \"--skip-dhctl-version-check\" flag may lead to cluster instability, data corruption, or complete cluster failure. We do NOT guarantee cluster operability when version check is skipped!",
			app.AppVersion, deckhouseVersion, deckhouseVersion)
	}

	return nil
}

func getDeckhouseVersionFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	releaseVersion, found, err := deckhouseVersionFromRelease(ctx, kubeCl)
	if err != nil {
		return "", err
	}
	if found {
		return releaseVersion, nil
	}

	deployVersion, err := deckhouseVersionFromDeployment(ctx, kubeCl)
	if err != nil {
		return "", err
	}
	if deployVersion == "" {
		return "", ErrDeckhouseVersionNotFound
	}

	return deployVersion, nil
}

func deckhouseVersionFromRelease(ctx context.Context, kubeCl *client.KubernetesClient) (string, bool, error) {
	list, err := kubeCl.Dynamic().Resource(deckhousev1alpha1.DeckhouseReleaseGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return "", false, nil
		}
		return "", false, err
	}

	for _, item := range list.Items {
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if phase != deckhousev1alpha1.PhaseDeployed {
			continue
		}
		version, _, _ := unstructured.NestedString(item.Object, "spec", "version")
		if version == "" {
			version = item.GetName()
		}
		if version == "" {
			continue
		}
		return version, true, nil
	}

	return "", false, nil
}

func deckhouseVersionFromDeployment(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	deployment, err := kubeCl.AppsV1().Deployments("d8-system").Get(ctx, "deckhouse", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", ErrDeckhouseVersionNotFound
		}
		return "", err
	}

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return "", fmt.Errorf("deckhouse deployment has no containers")
	}

	image := ""
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "deckhouse" {
			image = container.Image
			break
		}
	}
	if image == "" {
		return "", fmt.Errorf("deckhouse container not found in deployment")
	}

	imageTag, err := gcr.NewTag(image)
	if err != nil {
		return "", fmt.Errorf("deckhouse image has no tag: %s: %w", image, err)
	}

	return imageTag.TagStr(), nil
}

// versionsMatch compares two version strings using semver, handling different formats
// (e.g., "v1.73.0" vs "v1.73" vs "1.73.0")
func versionsMatch(v1, v2 string) bool {
	ver1, err := semver.NewVersion(strings.TrimPrefix(v1, "v"))
	if err != nil {
		return false
	}
	ver2, err := semver.NewVersion(strings.TrimPrefix(v2, "v"))
	if err != nil {
		return false
	}

	return ver1.Equal(ver2)
}
