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
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func TestBuildDestinationSinkLabelMaps_mergeExtraAddAndDrop(t *testing.T) {
	const src = v1alpha1.SourceFile

	tests := []struct {
		name           string
		extraLabels    map[string]string
		addLabelKeys   []string
		dropLabelPaths []string
		wantLoki       map[string]string
		wantSplunk     map[string]string
		wantCEF        map[string]string
	}{
		{
			name: "flat extra and add drops plus two source keys dropped",
			extraLabels: map[string]string{
				"team": "", "env": "", "svc": "", "rel": "", "build_id": "",
			},
			addLabelKeys: []string{"sink_a", "sink_b", "sink_c"},
			dropLabelPaths: []string{
				"team", "env", "svc", "sink_a", "sink_b",
				"host", "file",
			},
			wantLoki: map[string]string{
				"host_ip":  "{{ .host_ip }}",
				"rel":      "{{ rel }}",
				"build_id": "{{ build_id }}",
				"sink_c":   "{{ sink_c }}",
			},
			wantSplunk: map[string]string{
				"datetime": "",
				"host_ip":  "{{ .host_ip }}",
				"rel":      "{{ rel }}",
				"build_id": "{{ build_id }}",
				"sink_c":   "{{ sink_c }}",
			},
			wantCEF: map[string]string{
				"message":   "message",
				"timestamp": "timestamp",
				"hostip":    "host_ip",
				"rel":       "rel",
				"buildid":   "build_id",
				"sinkc":     "sink_c",
			},
		},
		{
			name:        "redundant add keys skipped under FilesLabels",
			extraLabels: map[string]string{"e1": "", "e2": "", "e3": "", "e4": ""},
			addLabelKeys: []string{
				"host", "file", "host.meta", "a1", "a2", "a3", "a4",
			},
			dropLabelPaths: []string{"e1", "e2", "e3", "a1", "a2", "host_ip"},
			wantLoki: map[string]string{
				"host": "{{ .host }}",
				"file": "{{ file }}",
				"e4":   "{{ e4 }}",
				"a3":   "{{ a3 }}",
				"a4":   "{{ a4 }}",
			},
			wantSplunk: map[string]string{
				"datetime": "",
				"host":     "{{ .host }}",
				"file":     "{{ file }}",
				"e4":       "{{ e4 }}",
				"a3":       "{{ a3 }}",
				"a4":       "{{ a4 }}",
			},
			wantCEF: map[string]string{
				"message":   "message",
				"timestamp": "timestamp",
				"host":      "host",
				"file":      "file",
				"e4":        "e4",
				"a3":        "a3",
				"a4":        "a4",
			},
		},
		{
			name:         "nested keys and one source field dropped",
			extraLabels:  map[string]string{"app.name": "", "app.id": "", "keep.top": "", "tier": "", "zone": ""},
			addLabelKeys: []string{"nest.x", "nest.y", "solo1", "solo2", "solo3"},
			dropLabelPaths: []string{
				"app", "nest", "tier", "solo1", "solo2",
				"file",
			},
			wantLoki: map[string]string{
				"host":     "{{ .host }}",
				"host_ip":  "{{ .host_ip }}",
				"keep.top": "{{ keep.top }}",
				"zone":     "{{ zone }}",
				"solo3":    "{{ solo3 }}",
			},
			wantSplunk: map[string]string{
				"datetime": "",
				"host":     "{{ .host }}",
				"host_ip":  "{{ .host_ip }}",
				"keep.top": "{{ keep.top }}",
				"zone":     "{{ zone }}",
				"solo3":    "{{ solo3 }}",
			},
			wantCEF: map[string]string{
				"message":   "message",
				"timestamp": "timestamp",
				"host":      "host",
				"hostip":    "host_ip",
				"keeptop":   "keep.top",
				"zone":      "zone",
				"solo3":     "solo3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extra := maps.Clone(tt.extraLabels)
			addLabelKeys := slicesClone(tt.addLabelKeys)
			drop := slicesClone(tt.dropLabelPaths)

			t.Run("Loki", func(t *testing.T) {
				spec := v1alpha1.ClusterLogDestinationSpec{
					Type:        v1alpha1.DestLoki,
					ExtraLabels: maps.Clone(extra),
				}
				got := BuildDestinationSinkLabelMaps(DestinationSinkInput{
					Spec: spec, SourceType: src, AddLabelKeys: addLabelKeys, DropLabelPaths: drop, WithPodLabels: true,
				})
				if diff := cmp.Diff(DestinationSinkLabelMaps{LokiLabels: tt.wantLoki}, got); diff != "" {
					t.Fatalf("BuildDestinationSinkLabelMaps Loki (-want +got):\n%s", diff)
				}
			})

			t.Run("Splunk", func(t *testing.T) {
				spec := v1alpha1.ClusterLogDestinationSpec{
					Type:        v1alpha1.DestSplunk,
					ExtraLabels: maps.Clone(extra),
				}
				got := BuildDestinationSinkLabelMaps(DestinationSinkInput{
					Spec: spec, SourceType: src, AddLabelKeys: addLabelKeys, DropLabelPaths: drop, WithPodLabels: false,
				})
				if diff := cmp.Diff(DestinationSinkLabelMaps{SplunkIndexedFields: tt.wantSplunk}, got); diff != "" {
					t.Fatalf("BuildDestinationSinkLabelMaps Splunk (-want +got):\n%s", diff)
				}
			})

			t.Run("Kafka_CEF", func(t *testing.T) {
				spec := v1alpha1.ClusterLogDestinationSpec{
					Type:        v1alpha1.DestKafka,
					ExtraLabels: maps.Clone(extra),
					Kafka: v1alpha1.KafkaSpec{
						Encoding: v1alpha1.CommonEncoding{Codec: v1alpha1.EncodingCodecCEF},
					},
				}
				got := BuildDestinationSinkLabelMaps(DestinationSinkInput{
					Spec: spec, SourceType: src, AddLabelKeys: addLabelKeys, DropLabelPaths: drop, WithPodLabels: false,
				})
				if diff := cmp.Diff(DestinationSinkLabelMaps{CEFExtensions: tt.wantCEF}, got); diff != "" {
					t.Fatalf("BuildDestinationSinkLabelMaps Kafka CEF (-want +got):\n%s", diff)
				}
			})

			t.Run("Socket_CEF", func(t *testing.T) {
				spec := v1alpha1.ClusterLogDestinationSpec{
					Type:        v1alpha1.DestSocket,
					ExtraLabels: maps.Clone(extra),
					Socket: v1alpha1.SocketSpec{
						Encoding: v1alpha1.CommonEncoding{Codec: v1alpha1.EncodingCodecCEF},
					},
				}
				got := BuildDestinationSinkLabelMaps(DestinationSinkInput{
					Spec: spec, SourceType: src, AddLabelKeys: addLabelKeys, DropLabelPaths: drop, WithPodLabels: false,
				})
				if diff := cmp.Diff(DestinationSinkLabelMaps{CEFExtensions: maps.Clone(tt.wantCEF)}, got); diff != "" {
					t.Fatalf("BuildDestinationSinkLabelMaps Socket CEF (-want +got):\n%s", diff)
				}
			})
		})
	}
}

func slicesClone[S ~[]E, E any](s S) S {
	if s == nil {
		return nil
	}
	return append(S(nil), s...)
}
