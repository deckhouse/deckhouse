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

const splunkIndexedFieldDatetime = "datetime"

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

var FilesLabels = map[string]string{
	"host":    "{{ .host }}",
	"host_ip": "{{ .host_ip }}",
	"file":    "{{ file }}",
}

type DestinationSinkArtifacts struct {
	LokiLabels          map[string]string
	SplunkIndexedFields map[string]string
	CEFExtensions       map[string]string
}

// DestinationSinkBuildInput is the single input for building Loki labels, Splunk indexed_fields, or CEF extensions.
type DestinationSinkInput struct {
	Spec           v1alpha1.ClusterLogDestinationSpec
	SourceType     string
	AddLabelKeys   []string
	DropLabelPaths []string
	WithPodLabels  bool
}

func (in DestinationSinkInput) sourceLabels() map[string]string {
	switch in.SourceType {
	case v1alpha1.SourceFile:
		return FilesLabels
	case v1alpha1.SourceKubernetesPods:
		if in.WithPodLabels {
			return K8sLabelsWithPodLabels
		}
		return K8sLabels
	default:
		return make(map[string]string)
	}
}

func (in DestinationSinkInput) orderedSinkKeys() []string {
	src := in.sourceLabels()
	extra := in.Spec.ExtraLabels
	keys := make([]string, 0, len(src)+len(extra)+len(in.AddLabelKeys))
	keys = append(keys, SortedMapKeys(src)...)
	keys = append(keys, SortedMapKeys(extra)...)
	for _, addKey := range in.AddLabelKeys {
		if addKeyRedundantWithList(keys, addKey) {
			continue
		}
		keys = append(keys, addKey)
	}
	return in.withoutDroppedKeys(keys)
}

func (in DestinationSinkInput) sinkMapKeyDropped(k string) bool {
	for _, dropPath := range in.DropLabelPaths {
		if sinkKeyMatchesDropPath(k, dropPath) {
			return true
		}
	}
	return false
}

func (in DestinationSinkInput) withoutDroppedKeys(keys []string) []string {
	if len(in.DropLabelPaths) == 0 {
		return keys
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !in.sinkMapKeyDropped(k) {
			out = append(out, k)
		}
	}
	return out
}

func (in DestinationSinkInput) splunkIndexedFields() map[string]string {
	base := sinkTemplateMapFromKeys(in, in.orderedSinkKeys())
	out := make(map[string]string, len(base)+1)
	maps.Copy(out, base)
	out[splunkIndexedFieldDatetime] = ""
	return out
}

func (in DestinationSinkInput) cefExtensions() map[string]string {
	keys := in.orderedSinkKeys()
	ext := make(map[string]string, len(keys)+2)
	ext["message"] = "message"
	ext["timestamp"] = "timestamp"
	for _, k := range keys {
		n := normalizeKey(k)
		if n == "cef.name" || n == "cef.severity" {
			continue
		}
		ext[n] = k
	}
	return ext
}

func BuildDestinationSinkArtifacts(in DestinationSinkInput) DestinationSinkArtifacts {
	spec := in.Spec
	var a DestinationSinkArtifacts
	switch spec.Type {
	case v1alpha1.DestLoki:
		a.LokiLabels = sinkTemplateMapFromKeys(in, in.orderedSinkKeys())
	case v1alpha1.DestSplunk:
		a.SplunkIndexedFields = in.splunkIndexedFields()
	case v1alpha1.DestKafka:
		if spec.Kafka.Encoding.Codec == v1alpha1.EncodingCodecCEF {
			a.CEFExtensions = in.cefExtensions()
		}
	case v1alpha1.DestSocket:
		if spec.Socket.Encoding.Codec == v1alpha1.EncodingCodecCEF {
			a.CEFExtensions = in.cefExtensions()
		}
	}
	return a
}

func SortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

var K8sLabelsWithPodLabels = func() map[string]string {
	result := make(map[string]string, len(K8sLabels)+1)
	maps.Copy(result, K8sLabels)
	result[podLabelsLokiKey] = "{{ pod_labels }}"
	return result
}()

func sinkTemplateMapFromKeys(in DestinationSinkInput, keys []string) map[string]string {
	src := in.sourceLabels()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := src[k]; ok {
			out[k] = v
		} else {
			out[k] = fieldRefTemplate(k)
		}
	}
	return out
}

func addKeyRedundantWithList(existingKeys []string, addKey string) bool {
	for _, existing := range existingKeys {
		root := existing
		if root == podLabelsLokiKey {
			root = K8sLabelPodLabels
		}
		if pathEqualsOrNestedUnder(addKey, root) {
			return true
		}
	}
	return false
}

func pathEqualsOrNestedUnder(label, path string) bool {
	if label == path {
		return true
	}
	return strings.HasPrefix(label, path+".")
}

func sinkKeyMatchesDropPath(sinkMapKey string, dropPath string) bool {
	if dropPath == "" {
		return false
	}
	candidate := sinkMapKey
	if candidate == podLabelsLokiKey {
		candidate = K8sLabelPodLabels
	}
	return pathEqualsOrNestedUnder(candidate, dropPath)
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
