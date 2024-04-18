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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/golang/protobuf/proto" // nolint: staticcheck
	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

// this hook checks helm releases (v2 and v3) and find deprecated apis
// hook returns only metrics:
//   `resource_versions_compatibility` for apis:
//      1 - is deprecated
//      2 - is unsupported in the next k8s version inside the delta interval
//		3 - is unsupported in the ANY next k8s version
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
	// fetchSecretsInterval pause between fetching the helm secrets from apiserver
	// need for avoiding apiserver overload
	fetchSecretsInterval = 3 * time.Second

	K8sVersionsWithDeprecations = "monitoringKubernetes:k8sVersionsWithDeprecations"
)

var helmStorage unsupportedVersionsStore

func init() {
	err := yaml.Unmarshal([]byte(unsupportedVersionsYAML), &helmStorage)
	if err != nil {
		log.Fatal(err)
	}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes/helm-releases-scan",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "helm_releases",
			Crontab: "0 * * * *", // every hour
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "kubernetesVersion",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        applyClusterConfigurationYamlFilter,
		},
	},
	// we don't need the startup hook, because this hook will start on synchronization
}, dependency.WithExternalDependencies(handleHelmReleases))

func applyClusterConfigurationYamlFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	var metaConfig *config.MetaConfig
	metaConfig, err = config.ParseConfigFromData(string(ccYaml))
	if err != nil {
		return nil, err
	}

	kubernetesVersion, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
	if err != nil {
		return nil, err
	}

	return kubernetesVersion, err
}
func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	err := json.Unmarshal(message, &result)
	return result, err
}

func handleHelmReleases(input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire("helm_deprecated_apiversions")

	k8sCurrentVersionRaw, ok := input.Values.GetOk("global.discovery.kubernetesVersion")
	if !ok {
		input.LogEntry.Warn("kubernetes version not found")
		return nil
	}
	k8sCurrentVersion := semver.MustParse(k8sCurrentVersionRaw.String())

	var isAutomaticK8s bool
	kubernetesVersion, ok := input.Snapshots["kubernetesVersion"]
	if ok && len(kubernetesVersion) > 0 && kubernetesVersion[0].(string) == "Automatic" {
		isAutomaticK8s = true
		requirements.SaveValue(K8sVersionsWithDeprecations, "initial")
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancelCause(context.Background())

	processor := &helmDeprecatedAPIsProcessor{
		logger: input.LogEntry,
		ctx:    ctx,
		cancel: cancel,
	}

	manifestsC := processor.FetchHelmManifests(client)
	defer func() {
		input.MetricsCollector.Set("helm_releases_count", float64(processor.totalHelm3Releases), map[string]string{"helm_version": "3"})
		input.MetricsCollector.Set("helm_releases_count", float64(processor.totalHelm2Releases), map[string]string{"helm_version": "2"})
	}()

	// helm3 and helm2 are listed and parsed in goroutines
	// deprecated resources will be processed here
	deprecations, err := runManifestsCheck(ctx, k8sCurrentVersion, input, manifestsC)
	if err != nil {
		return err
	}

	if isAutomaticK8s {
		if deprecations != "" {
			requirements.SaveValue(K8sVersionsWithDeprecations, deprecations)
		} else {
			requirements.RemoveValue(K8sVersionsWithDeprecations)
		}

		return nil
	}

	requirements.RemoveValue(K8sVersionsWithDeprecations)
	return nil
}

func runManifestsCheck(ctx context.Context, k8sCurrentVersion *semver.Version, input *go_hook.HookInput, manifestsC <-chan *manifestHead) (string, error) {
	allK8sWithDeprecations := make(map[string]struct{})

loop:
	for {
		select {
		case resource, ok := <-manifestsC:
			if !ok {
				break loop
			}

			incompatibility, k8sCompatibilityVersion := helmStorage.CalculateCompatibility(k8sCurrentVersion, resource.APIVersion, resource.Kind)
			switch incompatibility {
			case UpToDateVersion:
				// pass

			case UnsupportedVersion, DeprecatedVersion:
				allK8sWithDeprecations[k8sCompatibilityVersion] = struct{}{}
				input.MetricsCollector.Set("resource_versions_compatibility", float64(incompatibility), map[string]string{
					"helm_release_name":      resource.HelmReleaseInfo.Name,
					"helm_release_namespace": resource.HelmReleaseInfo.Namespace,
					"helm_version":           resource.HelmReleaseInfo.Version,
					"k8s_version":            k8sCompatibilityVersion,
					"resource_name":          resource.Metadata.Name,
					"resource_namespace":     resource.Metadata.Namespace,
					"kind":                   resource.Kind,
					"api_version":            resource.APIVersion,
				}, metrics.WithGroup("helm_deprecated_apiversions"))

			case FutureDeprecatedVersion:
				allK8sWithDeprecations[k8sCompatibilityVersion] = struct{}{}
			}

		case <-ctx.Done():
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			break loop
		}
	}

	result := strings.Builder{}
	for k := range allK8sWithDeprecations {
		result.WriteString(k + ",")
	}

	return strings.TrimSuffix(result.String(), ","), nil
}

type manifestHead struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Name      string `json:"name" yaml:"name"`
		Namespace string `json:"namespace" yaml:"namespace"`
	} `json:"metadata" yaml:"metadata"`

	HelmReleaseInfo struct {
		Name      string
		Namespace string
		Version   string
	} `json:"-" yaml:"-"`
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
	// DeprecatedVersion marks deprecated resources that will be removed in the next delta releases
	DeprecatedVersion
	UnsupportedVersion
	// FutureDeprecatedVersion resource that is deprecated in any future version
	FutureDeprecatedVersion
)

// CalculateCompatibility check compatibility. Returns
//
//	0 - if resource is compatible
//	1 - if resource in deprecated in the next delta(2) kubernetes versions and will be removed in the future
//	2 - if resource is unsupported for current k8s version
//	3 - if resource is deprecated in ANY k8s versions
func (uvs unsupportedVersionsStore) CalculateCompatibility(currentVersion *semver.Version, resourceAPIVersion, resourceKind string) (uint, string) {
	// check unsupported api for the current k8s version
	currentK8SAPIsStorage, exists := uvs.getByK8sVersion(currentVersion)
	if exists {
		isUnsupported := currentK8SAPIsStorage.isUnsupportedByAPIAndKind(resourceAPIVersion, resourceKind)
		if isUnsupported {
			return UnsupportedVersion, fmt.Sprintf("%d.%d", currentVersion.Major(), currentVersion.Minor())
		}
	}

	for version, store := range helmStorage {
		foundK8sVersion := semver.MustParse(version)
		if currentVersion.GreaterThan(foundK8sVersion) {
			// skip deprecation in previous k8s versions
			continue
		}
		if !store.isUnsupportedByAPIAndKind(resourceAPIVersion, resourceKind) {
			// skip up-to-date resources
			continue
		}

		resultString := fmt.Sprintf("%d.%d", foundK8sVersion.Major(), foundK8sVersion.Minor())
		switch {
		case foundK8sVersion.Minor() == currentVersion.Minor():
			return UnsupportedVersion, resultString

		case foundK8sVersion.Minor() <= currentVersion.Minor()+delta:
			return DeprecatedVersion, resultString

		case foundK8sVersion.Minor() > currentVersion.Minor()+delta:
			return FutureDeprecatedVersion, resultString
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

// Block with helm release deprecated api processor
type helmDeprecatedAPIsProcessor struct {
	totalHelm3Releases uint32
	totalHelm2Releases uint32
	logger             logger.Logger

	ctx    context.Context
	cancel context.CancelCauseFunc
}

func (h *helmDeprecatedAPIsProcessor) getHelm3Releases(client k8s.Client, releasesC chan<- *Release) error {
	var next string

	for {
		secretsList, err := client.CoreV1().Secrets("").List(h.ctx, metav1.ListOptions{
			LabelSelector: "owner=helm,status=deployed",
			Limit:         objectBatchSize,
			Continue:      next,
			// https://kubernetes.io/docs/reference/using-api/api-concepts/#semantics-for-get-and-list
			// set explicit behavior:
			//   Return data at any resource version. The newest available resource version is preferred, but strong consistency is not required; data at any resource version may be served.
			ResourceVersion:      "0",
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
		})
		if err != nil {
			return err
		}
		secretsList.GetRemainingItemCount()

		for _, secret := range secretsList.Items {
			releaseData := secret.Data["release"]
			if len(releaseData) == 0 {
				continue
			}

			release, err := helm3DecodeRelease(string(releaseData))
			if err != nil {
				return err
			}
			// release can contain wrong namespace (set by helm and werf) and confuse user with a wrong metric
			// fetch namespace from secret is more reliable
			release.Namespace = secret.Namespace
			release.HelmVersion = "3"

			releasesC <- release
			h.totalHelm3Releases++
		}

		if secretsList.Continue == "" {
			break
		}

		next = secretsList.Continue
		time.Sleep(fetchSecretsInterval)
	}

	return nil
}
func (h *helmDeprecatedAPIsProcessor) getHelm2Releases(client k8s.Client, releasesC chan<- *Release) error {
	var next string

	for {
		cmList, err := client.CoreV1().ConfigMaps("").List(h.ctx, metav1.ListOptions{
			LabelSelector:        "OWNER=TILLER,STATUS=DEPLOYED",
			Limit:                objectBatchSize,
			Continue:             next,
			ResourceVersion:      "0",
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
		})
		if err != nil {
			return err
		}

		for _, secret := range cmList.Items {
			releaseData := secret.Data["release"]
			if len(releaseData) == 0 {
				continue
			}

			release, err := helm2DecodeRelease(releaseData)
			if err != nil {
				return err
			}
			// release can contain wrong namespace (set by helm and werf) and confuse user with a wrong metric
			// fetch namespace from secret is more reliable
			release.Namespace = secret.Namespace
			release.HelmVersion = "2"

			releasesC <- release
			h.totalHelm2Releases++
		}

		if cmList.Continue == "" {
			break
		}

		next = cmList.Continue
		time.Sleep(fetchSecretsInterval)
	}

	return nil
}

func (h *helmDeprecatedAPIsProcessor) getHelmReleases(client k8s.Client) chan *Release {
	var (
		wg        sync.WaitGroup
		releasesC = make(chan *Release, objectBatchSize*2)
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := h.getHelm3Releases(client, releasesC)
		if err != nil {
			h.cancel(err)
			return
		}
	}()

	go func() {
		defer wg.Done()
		err := h.getHelm2Releases(client, releasesC)
		if err != nil {
			h.cancel(err)
			return
		}
	}()

	go func() {
		wg.Wait()
		close(releasesC)
	}()

	return releasesC
}

func (h *helmDeprecatedAPIsProcessor) FetchHelmManifests(client k8s.Client) chan *manifestHead {
	manifestsC := make(chan *manifestHead, objectBatchSize*2)
	releasesC := h.getHelmReleases(client)

	go func() {
		for rel := range releasesC {
			if h.ctx.Err() != nil {
				// return on cancelled context
				return
			}
			for _, manifestData := range releaseutil.SplitManifests(rel.Manifest) {
				resource := new(manifestHead)
				err := yaml.Unmarshal([]byte(manifestData), &resource)
				if err != nil {
					h.logger.Warnf("manifest (%s/%s) read error: %s", rel.Namespace, rel.Name, err)
					continue
				}

				if resource == nil {
					continue
				}

				resource.HelmReleaseInfo.Name = rel.Name
				resource.HelmReleaseInfo.Namespace = rel.Namespace
				resource.HelmReleaseInfo.Version = rel.HelmVersion

				manifestsC <- resource
			}
		}

		close(manifestsC)
	}()

	return manifestsC
}

// helm decoding
var magicGzip = []byte{0x1f, 0x8b, 0x08}

// Import this from helm3 lib - https://github.com/helm/helm/blob/49819b4ef782e80b0c7f78c30bd76b51ebb56dc8/pkg/storage/driver/util.go#L56
// helm3DecodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func helm3DecodeRelease(data string) (*Release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls Release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// https://github.com/helm/helm/blob/47f0b88409e71fd9ca272abc7cd762a56a1c613e/pkg/storage/driver/util.go#L57
func helm2DecodeRelease(data string) (*Release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls Release
	// unmarshal protobuf bytes
	if err := proto.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// protobuf methods for helm2
type Release struct {
	Name      string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name,proto3"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,8,opt,name=namespace,proto3"`
	Manifest  string `json:"manifest,omitempty" protobuf:"bytes,5,opt,name=manifest,proto3"`

	// set helm version manually
	HelmVersion string `json:"-"`
}

func (m *Release) Reset()         { *m = Release{} }
func (m *Release) String() string { return proto.CompactTextString(m) }
func (*Release) ProtoMessage()    {}
