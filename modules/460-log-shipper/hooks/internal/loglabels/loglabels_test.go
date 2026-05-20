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
	"slices"
	"testing"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func TestAppendAddLables(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		add  []string
		want []string
	}{
		{
			name: "skips duplicate and nested under existing host and skips message",
			keys: []string{"host", "file"},
			add:  []string{"host.meta", "file.meta", "a2", "message"},
			want: []string{"host", "file", "a2"},
		},
		{
			name: "skips message dot path and nested message fields",
			keys: []string{"x"},
			add:  []string{"message", "message.parsed", "y"},
			want: []string{"x", "y"},
		},
		{
			name: "skips nested under pod_labels_*",
			keys: []string{podLabelsLokiKey},
			add:  []string{"pod_labels.x", "solo"},
			want: []string{podLabelsLokiKey, "solo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendAddLables(tt.keys, tt.add)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if len(tt.add) == 0 && len(tt.keys) > 0 && len(got) > 0 && &got[0] != &tt.keys[0] {
				t.Fatalf("expected same slice when add is empty")
			}
		})
	}
}

func TestRemoveDropLables(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		drop []string
		want []string
	}{
		{
			name: "removes exact and nested under path",
			keys: []string{"host", "file", "app.name", "app.id", "keep.top", "zone"},
			drop: []string{"app", "file"},
			want: []string{"host", "keep.top", "zone"},
		},
		{
			name: "drop pod_labels removes pod_labels_*",
			keys: []string{"host", podLabelsLokiKey, "solo"},
			drop: []string{K8sLabelPodLabels},
			want: []string{"host", "solo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveDropLables(tt.keys, tt.drop)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if len(tt.drop) == 0 && len(tt.keys) > 0 && len(got) > 0 && &got[0] != &tt.keys[0] {
				t.Fatalf("expected same slice when drop is empty")
			}
		})
	}
}

func TestRemoveDropLablesThenAppendAddLablesKeepsKey(t *testing.T) {
	const src = v1alpha1.SourceFile
	keys := MergedSourceAndExtraLables(src, map[string]string{"keep": ""}, false)
	keys = RemoveDropLables(keys, []string{"host"})
	keys = AppendAddLables(keys, []string{"host"})
	want := []string{"file", "host_ip", "keep", "host"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
}
