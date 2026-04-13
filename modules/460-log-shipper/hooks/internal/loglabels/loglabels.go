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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
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
	K8sLabelStream     = "stream"
	K8sLabelNodeGroup  = "node_group"
)

const podLabelsLokiKey = "pod_labels_*"

// K8sLabels contains default Kubernetes labels for log destinations.
var K8sLabels = map[string]string{
	K8sLabelNamespace: "{{ namespace }}",
	K8sLabelContainer: "{{ container }}",
	K8sLabelImage:     "{{ image }}",
	K8sLabelPod:       "{{ pod }}",
	K8sLabelNode:      "{{ node }}",
	K8sLabelPodIP:     "{{ pod_ip }}",
	K8sLabelStream:    "{{ stream }}",
	K8sLabelNodeGroup: "{{ node_group }}",
	K8sLabelPodOwner:  "{{ pod_owner }}",
}

// FilesLabels contains default file labels for log destinations.
var FilesLabels = map[string]string{
	"host":    "{{ .host }}",
	"host_ip": "{{ .host_ip }}",
	"file":    "{{ file }}",
}

type DestinationSinkLabelMaps struct {
	LokiLabels               map[string]string
	SplunkIndexedFields      map[string]string
	CEFExtensions            map[string]string
	SyslogStructuredDataKeys []string
}

type DestinationSinkBuild struct {
	SourceType    string
	WithPodLabels bool
	Keys          []string
	Length        int
}

func (b DestinationSinkBuild) sinkTemplateMapFromKeys() map[string]string {
	src := sourceLabelsMap(b.SourceType, b.WithPodLabels)
	out := make(map[string]string, b.Length)
	for _, k := range b.Keys {
		if v, ok := src[k]; ok {
			out[k] = v
		} else {
			out[k] = fieldRefTemplate(k)
		}
	}
	return out
}

func (b DestinationSinkBuild) cefExtensionsFromKeys() map[string]string {
	ext := make(map[string]string, b.Length+2)
	ext["message"] = "message"
	ext["timestamp"] = "timestamp"
	for _, k := range b.Keys {
		n := normalizeKey(k)
		if n == "cef.name" || n == "cef.severity" {
			continue
		}
		ext[n] = k
	}
	return ext
}

func BuildDestinationSinkLabelMaps(spec v1alpha2.ClusterLogDestinationSpec, b DestinationSinkBuild) DestinationSinkLabelMaps {
	var a DestinationSinkLabelMaps
	switch spec.Type {
	case v1alpha1.DestLoki:
		a.LokiLabels = b.sinkTemplateMapFromKeys()
	case v1alpha1.DestSplunk:
		b.Length += 1
		base := b.sinkTemplateMapFromKeys()
		base["datetime"] = ""
		a.SplunkIndexedFields = base
	case v1alpha1.DestKafka:
		if spec.Kafka.Encoding.Codec == v1alpha2.EncodingCodecCEF {
			a.CEFExtensions = b.cefExtensionsFromKeys()
		}
	case v1alpha1.DestSocket:
		if spec.Socket.Encoding.Codec == v1alpha2.EncodingCodecCEF {
			a.CEFExtensions = b.cefExtensionsFromKeys()
		}
		if spec.Socket.Encoding.Codec == v1alpha2.EncodingCodecSyslog {
			a.SyslogStructuredDataKeys = syslogStructuredDataKeysSorted(b.Keys)
		}
	}
	return a
}

func syslogStructuredDataKeysSorted(labelKeys []string) []string {
	out := slices.Clone(labelKeys)
	slices.Sort(out)
	return out
}

func MergedSourceAndExtraLables(sourceType string, extra map[string]string, withPodLabels bool) []string {
	src := sourceLabelsMap(sourceType, withPodLabels)
	keys := make([]string, 0, len(src)+len(extra))
	keys = append(keys, SortedMapKeys(src)...)
	keys = append(keys, SortedMapKeys(extra)...)
	return keys
}

func sourceLabelsMap(sourceType string, withPodLabels bool) map[string]string {
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

func AppendAddLables(keys []string, addKeys []string) []string {
	if len(addKeys) == 0 {
		return keys
	}
	out := make([]string, len(keys), len(keys)+len(addKeys))
	copy(out, keys)
	for _, k := range addKeys {
		if pathEqualsOrNestedUnder(k, "message") {
			continue
		}
		if sinkLabelMatchesAnyCandidate(k, out) {
			continue
		}
		out = append(out, k)
	}
	return out
}

func RemoveDropLables(keys []string, dropKeys []string) []string {
	if len(dropKeys) == 0 {
		return keys
	}
	kept := make([]string, 0, len(keys))
	for _, k := range keys {
		if sinkLabelMatchesAnyCandidate(k, dropKeys) {
			continue
		}
		kept = append(kept, k)
	}
	return kept
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

// K8sLabelsWithPodLabels contains K8sLabels plus pod_labels_*.
var K8sLabelsWithPodLabels = func() map[string]string {
	result := make(map[string]string, len(K8sLabels)+1)
	maps.Copy(result, K8sLabels)
	result[podLabelsLokiKey] = "{{ pod_labels }}"
	return result
}()

// Convert Loki pod labels wildcard key to the real key.
// pod_labels_* → pod_labels
func lokiPodLabels(k string) string {
	if k == podLabelsLokiKey {
		return K8sLabelPodLabels
	}
	return k
}

func pathEqualsOrNestedUnder(label, path string) bool {
	if label == path {
		return true
	}
	return strings.HasPrefix(label, path+".")
}

func sinkLabelMatchesAnyCandidate(label string, candidates []string) bool {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if pathEqualsOrNestedUnder(lokiPodLabels(label), lokiPodLabels(c)) {
			return true
		}
	}
	return false
}

func fieldRefTemplate(key string) string {
	return fmt.Sprintf("{{ %s }}", key)
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
