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
	"file":    "{{ file }}",
}

// SortedMapKeys returns sorted keys from a map for deterministic order.
func SortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// GetLokiLabels returns labels for Loki: source labels (with pod_labels for K8s) + extraLabels.
func GetLokiLabels(sourceType string, extraLabels map[string]string) map[string]string {
	return mergeLabels(sourceLabels(sourceType, true), extraLabels)
}

// GetSplunkLabels returns indexed fields for Splunk: datetime + source labels (K8s without pod_labels) + extraLabels.
func GetSplunkLabels(sourceType string, extraLabels map[string]string) map[string]string {
	result := make(map[string]string, 1+len(FilesLabels)+len(extraLabels))
	result["datetime"] = ""
	maps.Copy(result, mergeLabels(sourceLabels(sourceType, false), extraLabels))
	return result
}

// GetSyslogLabels returns sorted label keys for syslog (for deterministic VRL order).
func GetSyslogLabels(sourceType string, extraLabels map[string]string) []string {
	return SortedMapKeys(mergeLabels(sourceLabels(sourceType, false), extraLabels))
}

// GetCEFExtensions returns CEF extensions map: source labels (K8s without pod_labels) + extraLabels, with message/timestamp.
func GetCEFExtensions(sourceType string, extraLabels map[string]string) map[string]string {
	sourceLabels := sourceLabels(sourceType, false)
	extensions := make(map[string]string, len(sourceLabels)+len(extraLabels)+5)
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
		extensions[normalizeKey(k)] = k
	}
	for _, k := range SortedMapKeys(extraLabels) {
		n := normalizeKey(k)
		if n == "cef.name" || n == "cef.severity" {
			continue
		}
		extensions[n] = k
	}
	return extensions
}

func sourceLabels(sourceType string, withPodLabels bool) map[string]string {
	switch sourceType {
	case v1alpha1.SourceFile:
		return FilesLabels
	case v1alpha1.SourceKubernetesPods:
		if withPodLabels {
			return K8sLabelsWithPodLabels
		}
		return K8sLabels
	default:
		return make(map[string]string)
	}
}

func mergeLabels(sourceLabels map[string]string, extraLabels map[string]string) map[string]string {
	result := make(map[string]string, len(sourceLabels)+len(extraLabels))
	maps.Copy(result, sourceLabels)
	for _, k := range SortedMapKeys(extraLabels) {
		result[k] = fmt.Sprintf("{{ %s }}", k)
	}
	return result
}

func normalizeKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		if unicode.IsLetter(c) || unicode.IsNumber(c) {
			b.WriteRune(unicode.ToLower(c))
		}
	}
	return b.String()
}
