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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// VersionCheckOptions contains options for version compatibility checking.
type VersionCheckOptions struct {
	// AllowAnyError allows the check to continue even if version check fails.
	// This is useful for destroy operations where cluster might be partially deleted.
	AllowAnyError bool
	// AllowMissingVersion allows the check to pass if version cannot be determined from cluster.
	AllowMissingVersion bool
}

// CheckDeckhouseVersionCompatibility validates that dhctl version matches Deckhouse version in cluster.
// Returns an error if versions don't match or if version cannot be determined (unless AllowMissingVersion is true).
func CheckDeckhouseVersionCompatibility(ctx context.Context, kubeCl *client.KubernetesClient, opts VersionCheckOptions) error {
	dhctlVersion := app.AppVersion

	// Skip check for local/dev builds
	if dhctlVersion == "local" || dhctlVersion == "dev" {
		log.InfoF("Skipping version check for dhctl version: %s\n", dhctlVersion)
		return nil
	}

	deckhouseVersion, err := getDeckhouseVersionFromCluster(ctx, kubeCl)
	if err != nil {
		if opts.AllowAnyError {
			log.WarnF("Could not determine Deckhouse version from cluster: %v\n", err)
			log.WarnLn("Continuing despite version check failure (allowAnyError=true)")
			return nil
		}
		if opts.AllowMissingVersion {
			log.WarnF("Could not determine Deckhouse version from cluster: %v\n", err)
			log.WarnLn("Continuing despite version check failure (allowMissingVersion=true)")
			return nil
		}
		return fmt.Errorf("failed to get Deckhouse version from cluster: %w", err)
	}

	if deckhouseVersion == "" {
		if opts.AllowMissingVersion {
			log.WarnLn("Deckhouse version not found in cluster, skipping version check")
			return nil
		}
		return fmt.Errorf("Deckhouse version not found in cluster")
	}

	// Normalize versions for comparison (remove 'v' prefix if present)
	normalizedDhctlVersion := normalizeVersion(dhctlVersion)
	normalizedDeckhouseVersion := normalizeVersion(deckhouseVersion)

	if normalizedDhctlVersion != normalizedDeckhouseVersion {
		errMsg := fmt.Sprintf(
			"Version mismatch detected!\n"+
				"  dhctl version:     %s\n"+
				"  Deckhouse version:  %s\n"+
				"\n"+
				"This version mismatch can cause critical errors.\n"+
				"Please use dhctl with version tag matching your Deckhouse version.\n"+
				"For example, if your Deckhouse version is %s, use: dhctl:%s",
			dhctlVersion, deckhouseVersion, deckhouseVersion, deckhouseVersion,
		)

		if opts.AllowAnyError {
			log.WarnLn(errMsg)
			log.WarnLn("Continuing despite version mismatch (allowAnyError=true)")
			return nil
		}

		return errors.New(errMsg)
	}

	log.InfoF("Version check passed: dhctl %s matches Deckhouse %s\n", dhctlVersion, deckhouseVersion)
	return nil
}

// getDeckhouseVersionFromCluster attempts to get Deckhouse version from cluster.
// First tries to get version from DeckhouseRelease CRD (phase == "Deployed"),
// then falls back to deployment image tag.
func getDeckhouseVersionFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	// Try to get version from DeckhouseRelease CRD first
	version, releaseErr := getDeckhouseVersionFromRelease(ctx, kubeCl)
	if releaseErr == nil && version != "" {
		return version, nil
	}

	// Fallback to deployment image tag
	version, deployErr := getDeckhouseVersionFromDeployment(ctx, kubeCl)
	if deployErr == nil && version != "" {
		return version, nil
	}

	// Combine errors if both failed
	if releaseErr != nil && deployErr != nil {
		return "", fmt.Errorf("could not determine Deckhouse version: tried release (%v), deployment (%w)", releaseErr, deployErr)
	}
	if releaseErr != nil {
		return "", fmt.Errorf("could not determine Deckhouse version from release: %w", releaseErr)
	}
	return "", fmt.Errorf("could not determine Deckhouse version from deployment: %w", deployErr)
}

// getDeckhouseVersionFromRelease gets Deckhouse version from DeckhouseRelease CRD.
// Returns version from the release with phase == "Deployed".
func getDeckhouseVersionFromRelease(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	gvr := v1alpha1.DeckhouseReleaseGVR

	// List all DeckhouseRelease resources
	unstructuredList, err := kubeCl.Dynamic().Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			// CRD might not exist or we might not have permissions - this is ok, fallback to deployment
			return "", fmt.Errorf("DeckhouseRelease CRD not accessible: %w", err)
		}
		return "", fmt.Errorf("list DeckhouseRelease resources: %w", err)
	}

	// Find the release with phase == "Deployed"
	for _, item := range unstructuredList.Items {
		phase, found, err := unstructured.NestedString(item.Object, "status", "phase")
		if err != nil || !found {
			continue
		}

		if phase == v1alpha1.PhaseDeployed {
			version, found, err := unstructured.NestedString(item.Object, "spec", "version")
			if err != nil || !found {
				continue
			}
			if version != "" {
				return version, nil
			}
		}
	}

	return "", fmt.Errorf("no DeckhouseRelease with phase 'Deployed' found")
}

// getDeckhouseVersionFromDeployment gets Deckhouse version from deployment image tag.
func getDeckhouseVersionFromDeployment(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	const (
		namespace = "d8-system"
		name      = "deckhouse"
	)

	deployment, err := kubeCl.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("deckhouse deployment not found in namespace %s", namespace)
		}
		return "", fmt.Errorf("get deckhouse deployment: %w", err)
	}

	// Get version from container image
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return "", fmt.Errorf("deckhouse deployment has no containers")
	}

	containerImage := deployment.Spec.Template.Spec.Containers[0].Image
	version := extractImageTag(containerImage)
	if version == "" {
		return "", fmt.Errorf("could not extract version from container image: %s", containerImage)
	}

	return version, nil
}

// extractImageTag extracts version tag from container image string.
// Handles formats like:
//   - registry.deckhouse.io/deckhouse/ce:v1.73.0
//   - registry.deckhouse.io/deckhouse/ce@sha256:abc123...
//   - registry.deckhouse.io/deckhouse/ce:v1.73.0@sha256:abc123...
func extractImageTag(image string) string {
	// Handle digest format: image@sha256:...
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}

	// Extract tag after last ':'
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return ""
	}

	tag := image[lastColon+1:]
	return tag
}

// normalizeVersion normalizes version string by removing 'v' prefix if present.
// Examples: "v1.73.0" -> "1.73.0", "1.73.0" -> "1.73.0"
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	// Basic validation: versions should contain at least one dot (e.g., "1.73.0")
	// If version is empty or doesn't contain a dot, return as-is to let comparison fail naturally
	if version == "" {
		return version
	}
	// Note: We don't enforce strict format here to allow flexibility,
	// but empty versions will be caught by the caller's validation
	return version
}
