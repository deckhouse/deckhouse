/*
Copyright 2021 Flant JSC

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
	"log"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/kube-client/manifest/releaseutil" // nolint: staticcheck
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	helmreleases "github.com/deckhouse/deckhouse/modules/340-monitoring-kubernetes/hooks/internal"
)

// this hook checks helm releases (v2 and v3) and find deprecated apis
// hook returns only metrics:
//   `resource_versions_compatibility` for apis:
//      1 - is deprecated
//      2 - in unsupported
// Also hook returns count on deployed releases `helm_releases_count`
// Hook checks only releases with status: deployed

// **Attention**
// Releases are checked via kubeclient not by snapshots to avoid huge memory consumption
// on some installations snapshots can take gigabytes of memory. Releases are checked by batches with size specified in
// objectBatchSize. It means, that kubeClient will list only limited amount of releases to avoid memory explosion

const unsupportedVersionsYAML = `
"1.22":
  "admissionregistration.k8s.io/v1beta1": ["ValidatingWebhookConfiguration", "MutatingWebhookConfiguration"]
  "apiextensions.k8s.io/v1beta1": ["CustomResourceDefinition"]
  "apiregistration.k8s.io/v1beta1": ["APIService"]
  "authentication.k8s.io/v1beta1": ["TokenReview"]
  "authorization.k8s.io/v1beta1": ["SubjectAccessReview", "LocalSubjectAccessReview", "SelfSubjectAccessReview"]
  "certificates.k8s.io/v1beta1": ["CertificateSigningRequest"]
  "coordination.k8s.io/v1beta1": ["Lease"]
  "networking.k8s.io/v1beta1": ["Ingress"]
  "extensions/v1beta1": ["Ingress"]

"1.24":
  "snapshot.storage.k8s.io/v1beta1": ["VolumeSnapshot"]

"1.25":
  "batch/v1beta1": ["CronJob"]
  "discovery.k8s.io/v1beta1": ["EndpointSlice"]
  "events.k8s.io/v1beta1": ["Event"]
  "autoscaling/v2beta1": ["HorizontalPodAutoscaler"]
  "policy/v1beta1": ["PodDisruptionBudget", "PodSecurityPolicy"]
  "node.k8s.io/v1beta1": ["RuntimeClass"]

"1.26":
  "flowcontrol.apiserver.k8s.io/v1beta1": ["FlowSchema", "PriorityLevelConfiguration"]
  "autoscaling/v2beta2": ["HorizontalPodAutoscaler"]

"1.27":
  "storage.k8s.io/v1beta1": ["CSIStorageCapacity"]

"1.29":
  "flowcontrol.apiserver.k8s.io/v1beta2": ["FlowSchema", "PriorityLevelConfiguration"]

"1.32":
  "flowcontrol.apiserver.k8s.io/v1beta3": ["FlowSchema", "PriorityLevelConfiguration"]
`

const (
	// delta for k8s versions which are checked for deprecated apis
	// with delta == 2 for k8s 1.21 will also check apis for 1.22 and 1.23
	delta = 2
	// objectBatchSize - how many secrets to list from k8s at once
	objectBatchSize = int64(10)
)

var helmStorage unsupportedVersionsStore

func init() {
	err := yaml.Unmarshal([]byte(unsupportedVersionsYAML), &helmStorage)
	if err != nil {
		log.Fatal(err)
	}
}

// maximum time deep for cached releases. Variable required for overriding in tests.
var helmReleasesInterval = helmreleases.IntervalHours1

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes/helm_releases",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "helm_releases",
			Crontab: "0 * * * *", // every hour
		},
	},
	OnStartup: &go_hook.OrderedConfig{
		Order: 1,
	},
}, dependency.WithExternalDependencies(handleHelmReleasesBySchedule))

func handleHelmReleasesBySchedule(input *go_hook.HookInput, dc dependency.Container) error {
	return handleHelmReleases(input, dc, helmReleasesInterval)
}

func handleHelmReleases(input *go_hook.HookInput, dc dependency.Container, interval helmreleases.Interval) error {
	var (
		totalHelm3Releases uint32
		totalHelm2Releases uint32
	)

	input.MetricsCollector.Expire("helm_deprecated_apiversions")

	k8sCurrentVersionRaw, ok := input.Values.GetOk("global.discovery.kubernetesVersion")
	if !ok {
		input.LogEntry.Warn("kubernetes version not found")
		return nil
	}
	k8sCurrentVersion := semver.MustParse(k8sCurrentVersionRaw.String())

	// create buffered channel == objectBatchSize
	// this give as ability to handle in memory only objectBatchSize * 2 amount of helm releases
	// because this counter also used as a limit to apiserver
	// we have `objectBatchSize` (25) objects in channel and max `objectBatchSize` (25) objects in goroutine waiting for channel
	releasesC := make(chan *helmreleases.Release, objectBatchSize)
	doneC := make(chan bool)

	// helm3 and helm2 are listed and parsed in goroutines
	// deprecated resources will be processed here in the separated goroutine
	go runReleaseProcessor(k8sCurrentVersion, input, releasesC, doneC)

	ctx := context.Background()
	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	go func() {
		totalHelm3Releases, totalHelm2Releases, err = helmreleases.GetHelmReleases(ctx, client, releasesC, interval)
	}()
	<-doneC

	// to avoid data race
	input.MetricsCollector.Set("helm_releases_count", float64(totalHelm3Releases), map[string]string{"helm_version": "3"})
	input.MetricsCollector.Set("helm_releases_count", float64(totalHelm2Releases), map[string]string{"helm_version": "2"})

	return nil
}

func runReleaseProcessor(k8sCurrentVersion *semver.Version, input *go_hook.HookInput, releasesC <-chan *helmreleases.Release, doneC chan<- bool) {
	defer func() {
		doneC <- true
	}()
	for rel := range releasesC {
		for _, manifestData := range releaseutil.SplitManifests(rel.Manifest) {
			resource := new(manifest)
			err := yaml.Unmarshal([]byte(manifestData), &resource)
			if err != nil {
				input.LogEntry.Errorf("manifest (%s/%s) read error: %s", rel.Namespace, rel.Name, err)
				continue
			}

			if resource == nil {
				continue
			}

			incompatibility, k8sCompatibilityVersion := helmStorage.CalculateCompatibility(k8sCurrentVersion, resource.APIVersion, resource.Kind)
			if incompatibility > 0 {
				input.MetricsCollector.Set("resource_versions_compatibility", float64(incompatibility), map[string]string{
					"helm_release_name":      rel.Name,
					"helm_release_namespace": rel.Namespace,
					"helm_version":           rel.HelmVersion,
					"k8s_version":            k8sCompatibilityVersion,
					"resource_name":          resource.Metadata.Name,
					"resource_namespace":     resource.Metadata.Namespace,
					"kind":                   resource.Kind,
					"api_version":            resource.APIVersion,
				}, metrics.WithGroup("helm_deprecated_apiversions"))
			}
		}
	}
}

type manifest struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Name      string `json:"name" yaml:"name"`
		Namespace string `json:"namespace" yaml:"namespace"`
	} `json:"metadata" yaml:"metadata"`
}

// k8s version: APIVersion: [Kind, ...]
type unsupportedVersionsStore map[string]unsupportedAPIVersions

func (uvs unsupportedVersionsStore) getByK8sVersion(version *semver.Version) (unsupportedAPIVersions, bool) {
	majorMinor := fmt.Sprintf("%d.%d", version.Major(), version.Minor())
	apis, ok := uvs[majorMinor]
	return apis, ok
}

const (
	UpToDateVersion uint = iota
	DeprecatedVersion
	UnsupportedVersion
)

// CalculateCompatibility check compatibility. Returns
//
//	 0 - if resource is compatible
//	 1 - if resource in deprecated and will be removed in the future
//	 2 - if resource is unsupported for current k8s version
//	and k8s version in which deprecation would be
func (uvs unsupportedVersionsStore) CalculateCompatibility(currentVersion *semver.Version, resourceAPIVersion, resourceKind string) (uint, string) {
	// check unsupported api for the current k8s version
	currentK8SAPIsStorage, exists := uvs.getByK8sVersion(currentVersion)
	if exists {
		isUnsupported := currentK8SAPIsStorage.isUnsupportedByAPIAndKind(resourceAPIVersion, resourceKind)
		if isUnsupported {
			return UnsupportedVersion, fmt.Sprintf("%d.%d", currentVersion.Major(), currentVersion.Minor())
		}
	}

	// if api is supported - check deprecation in the next 2 minor k8s versions
	for i := 1; i <= delta; i++ {
		newMinor := currentVersion.Minor() + uint64(i)
		nextVersion := semver.MustParse(fmt.Sprintf("%d.%d.0", currentVersion.Major(), newMinor))
		storage, exists := uvs.getByK8sVersion(nextVersion)
		if exists {
			isDeprecated := storage.isUnsupportedByAPIAndKind(resourceAPIVersion, resourceKind)
			if isDeprecated {
				return DeprecatedVersion, fmt.Sprintf("%d.%d", nextVersion.Major(), nextVersion.Minor())
			}
		}
	}

	return UpToDateVersion, ""
}

// APIVersion: [Kind]
type unsupportedAPIVersions map[string][]string

func (ua unsupportedAPIVersions) isUnsupportedByAPIAndKind(resourceAPI, resourceKind string) bool {
	kinds, ok := ua[resourceAPI]
	if !ok {
		return false
	}
	for _, kind := range kinds {
		if kind == resourceKind {
			return true
		}
	}

	return false
}
