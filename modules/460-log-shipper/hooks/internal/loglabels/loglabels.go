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

package loglabels

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

// Kubernetes label field names used in kubernetes_logs source annotation_fields.
const (
	K8sLabelPod        = "pod"
	K8sLabelPodLabels  = "pod_labels"
	K8sLabelPodIP      = "pod_ip"
	K8sLabelNamespace  = "namespace"
	K8sLabelImage      = "image"
	K8sLabelContainer  = "container"
	K8sLabelNode       = "node"
	K8sLabelPodOwner   = "pod_owner"
	K8sLabelNodeLabels = "node_labels"
)

const podLabelsStar = "pod_labels_*"

// K8sLabels contains default Kubernetes labels for log destinations.
var K8sLabels = map[string]string{
	K8sLabelNamespace: "{{ namespace }}",
	K8sLabelContainer: "{{ container }}",
	K8sLabelImage:     "{{ image }}",
	K8sLabelPod:       "{{ pod }}",
	K8sLabelNode:      "{{ node }}",
	K8sLabelPodIP:     "{{ pod_ip }}",
	"stream":          "{{ stream }}",
	"node_group":      "{{ node_group }}",
	K8sLabelPodOwner:  "{{ pod_owner }}",
}

// K8sLabelsWithPodLabels contains K8sLabels plus pod_labels_*.
var K8sLabelsWithPodLabels = func() map[string]string {
	result := make(map[string]string, len(K8sLabels)+1)
	maps.Copy(result, K8sLabels)
	result[podLabelsStar] = "{{ pod_labels }}"
	return result
}()

// FilesLabels contains default file labels for log destinations.
var FilesLabels = map[string]string{
	"host":    "{{ .host }}",
	"host_ip": "{{ .host_ip }}",
	"file":    "{{ .file }}",
}

// SortedExtraLabelsKeys returns sorted keys from extraLabels map.
func SortedExtraLabelsKeys(extraLabels map[string]string) []string {
	keys := make([]string, 0, len(extraLabels))
	for key := range extraLabels {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// NormalizeKey normalizes a key by keeping only letters and numbers, converting to lowercase.
func NormalizeKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		if unicode.IsLetter(c) || unicode.IsNumber(c) {
			b.WriteRune(unicode.ToLower(c))
		}
	}
	return b.String()
}

// MergeLabels merges source labels and extraLabels into a single map.
func MergeLabels(sourceLabels map[string]string, extraLabels map[string]string) map[string]string {
	result := make(map[string]string, len(sourceLabels)+len(extraLabels))
	maps.Copy(result, sourceLabels)
	for _, k := range SortedExtraLabelsKeys(extraLabels) {
		result[k] = fmt.Sprintf("{{ %s }}", k)
	}
	return result
}

// MergeLabelsForSource merges labels based on source type and extraLabels.
func MergeLabelsForSource(sourceType string, extraLabels map[string]string) map[string]string {
	var sourceLabels map[string]string
	switch sourceType {
	case v1alpha1.SourceFile:
		sourceLabels = FilesLabels
	case v1alpha1.SourceKubernetesPods:
		sourceLabels = K8sLabelsWithPodLabels
	default:
		sourceLabels = make(map[string]string)
	}
	return MergeLabels(sourceLabels, extraLabels)
}

// GetCEFExtensionsForLabels returns CEF extensions map based on source labels.
func GetCEFExtensionsForLabels(sourceLabels map[string]string) map[string]string {
	extensions := make(map[string]string, len(sourceLabels)+3)
	extensions["message"] = "message"
	extensions["timestamp"] = "timestamp"
	for k := range sourceLabels {
		if k == podLabelsStar {
			continue
		}
		if k == K8sLabelNode {
			extensions["node"] = "node"
			continue
		}
		normalized := NormalizeKey(k)
		extensions[normalized] = k
	}
	return extensions
}

// GetCEFExtensionsForSource returns CEF extensions map based on source type.
func GetCEFExtensionsForSource(sourceType string) map[string]string {
	var sourceLabels map[string]string
	switch sourceType {
	case v1alpha1.SourceFile:
		sourceLabels = FilesLabels
	case v1alpha1.SourceKubernetesPods:
		sourceLabels = K8sLabels
	default:
		sourceLabels = make(map[string]string)
	}
	return GetCEFExtensionsForLabels(sourceLabels)
}

var (
	cefSpecialKeys = map[string]struct{}{
		"cef.name":     {},
		"cef.severity": {},
	}
)

// GetCEFExtensionsWithExtraLabels returns CEF extensions with extraLabels added, excluding special keys.
func GetCEFExtensionsWithExtraLabels(sourceType string, extraLabels map[string]string) map[string]string {
	extensions := GetCEFExtensionsForSource(sourceType)
	for _, k := range SortedExtraLabelsKeys(extraLabels) {
		normalized := NormalizeKey(k)
		if _, isSpecial := cefSpecialKeys[normalized]; isSpecial {
			continue
		}
		extensions[normalized] = k
	}
	return extensions
}
