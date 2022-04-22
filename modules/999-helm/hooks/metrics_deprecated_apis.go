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
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/helm/helm_releases",
	OnStartup: &go_hook.OrderedConfig{
		Order: 1,
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "helm_releases",
			Crontab: "*/20 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "helm_release_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"owner": "helm",
				},
			},
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			WaitForSynchronization:       pointer.BoolPtr(false),
			FilterFunc:                   filterHelmSecret,
		},
	},
}, handleHelmSecrets)

func filterHelmSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	releaseData := sec.Data["release"]
	if len(releaseData) == 0 {
		return nil, nil
	}

	return decodeRelease(string(releaseData))
}

func handleHelmSecrets(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("helm_deprecated_apiversions")

	// TODO: get k8s version
	k8sCurrentVersionRaw, ok := input.Values.GetOk("global.discovery.kubernetesVersion")
	if !ok {
		input.LogEntry.Warn("kubernetes version not found")
		return nil
	}
	k8sCurrentVersion := k8sCurrentVersionRaw.String()
	// TODO: get deprecations for this version
	versionStorage, exists := storage.getByK8sVersion(k8sCurrentVersion)
	if !exists {
		input.LogEntry.Infof("No Deprecated API versions for %s", k8sCurrentVersion)
		return nil
	}

	// get helm manifests
	snap := input.Snapshots["helm_release_secrets"]
	for _, sn := range snap {
		if sn == nil {
			continue
		}

		helmRelease := sn.(*release)

		arr := strings.Split(helmRelease.Manifest, "---")
		if len(arr) < 1 {
			continue
		}
		for _, resourceRaw := range arr[1:] {
			var resource manifest
			err := yaml.Unmarshal([]byte(resourceRaw), &resource)
			if err != nil {
				return err
			}

			isUnsupported := versionStorage.isUnsupportedByAPIAndKind(resource.APIVersion, resource.Kind)
			if isUnsupported {
				input.MetricsCollector.Set("resource_versions_compatibility", 1, map[string]string{
					"helm_release_name":      helmRelease.Name,
					"helm_release_namespace": helmRelease.Namespace,

					"resource_name": resource.Metadata.Name,
					"kind":          resource.Kind,
					"api_version":   resource.APIVersion,
					"namespace":     resource.Metadata.Namespace,
				}, metrics.WithGroup("helm_deprecated_apiversions"))
			}
		}
	}
	return nil
}

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// decodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func decodeRelease(data string) (*release, error) {
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
		b2, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

type release struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Manifest  string `json:"manifest,omitempty"`
}

type manifest struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Name      string `json:"name" yaml:"name"`
		Namespace string `json:"namespace" yaml:"namespace"`
	} `json:"metadata" yaml:"metadata"`
}

// k8s version: APIVersion: Kind
// example: "1.22": {"networking.k8s.io/v1beta1": ["Ingress"]}
type unsupportedVersionsStore map[string]unsupportedApiVersions

func (uvs unsupportedVersionsStore) getByK8sVersion(version string) (unsupportedApiVersions, bool) {
	arr := strings.Split(version, ".")
	majorMinor := arr[0] + "." + arr[1]
	apis, ok := uvs[majorMinor]
	return apis, ok
}

type unsupportedApiVersions map[string][]string

func (ua unsupportedApiVersions) isUnsupportedByAPIAndKind(api, ikind string) bool {
	kinds, ok := ua[api]
	if !ok {
		return false
	}
	for _, kind := range kinds {
		if kind == ikind {
			return true
		}
	}

	return false
}

var unsupportedVersionsYAML = `
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

"1.25":
  "batch/v1beta1": ["CronJob"]
  "discovery.k8s.io/v1beta1": ["EndpointSlice"]
  "events.k8s.io/v1beta1": ["Event"]
  "autoscaling/v2beta1": ["HorizontalPodAutoscaler"]
  "policy/v1beta1": ["PodDisruptionBudget", "PodSecurityPolicy"]
  "node.k8s.io/v1beta1": ["RuntimeClass"]
`

var storage unsupportedVersionsStore

func init() {
	err := yaml.Unmarshal([]byte(unsupportedVersionsYAML), &storage)
	if err != nil {
		log.Fatal(err)
	}
}
