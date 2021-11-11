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

package vector

import (
	"testing"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

func TestDestPatch(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		gen := NewLogConfigGenerator()

		source1 := NewKubernetesLogSource("s1", v1alpha1.KubernetesPodsSpec{}, false)
		source2 := NewKubernetesLogSource("s2", v1alpha1.KubernetesPodsSpec{}, false)
		source3 := NewKubernetesLogSource("s3", v1alpha1.KubernetesPodsSpec{}, false)
		dest := NewLokiDestination("d1", v1alpha1.ClusterLogDestinationSpec{})

		gen.AppendLogPipeline(source1, nil, []impl.LogDestination{dest})
		gen.AppendLogPipeline(source2, nil, []impl.LogDestination{dest})
		gen.AppendLogPipeline(source3, nil, []impl.LogDestination{dest})

		require.Len(t, gen.destinations, 1)
		assert.Equal(t, gen.destinations[0].(*lokiDestination).Inputs, []string{"d8_cluster_source_s1", "d8_cluster_source_s2", "d8_cluster_source_s3"})
	})

	t.Run("cluster source with namespace", func(t *testing.T) {
		gen := NewLogConfigGenerator()

		source1 := NewKubernetesLogSource("s1", v1alpha1.KubernetesPodsSpec{}, false)
		source2 := NewKubernetesLogSource("s2", v1alpha1.KubernetesPodsSpec{NamespaceSelector: types.NameSelector{MatchNames: []string{"n1", "n2"}}}, false)
		dest := NewLokiDestination("d1", v1alpha1.ClusterLogDestinationSpec{})

		gen.AppendLogPipeline(source1, nil, []impl.LogDestination{dest})
		gen.AppendLogPipeline(source2, nil, []impl.LogDestination{dest})

		require.Len(t, gen.destinations, 1)
		assert.Equal(t, gen.destinations[0].(*lokiDestination).Inputs, []string{"d8_cluster_source_s1", "d8_clusterns_source_n1_s2", "d8_clusterns_source_n2_s2"})
	})

	t.Run("Namespaced", func(t *testing.T) {
		gen := NewLogConfigGenerator()

		source1 := NewKubernetesLogSource("s1", v1alpha1.KubernetesPodsSpec{NamespaceSelector: types.NameSelector{MatchNames: []string{"ns1"}}}, true)
		source2 := NewKubernetesLogSource("s2", v1alpha1.KubernetesPodsSpec{NamespaceSelector: types.NameSelector{MatchNames: []string{"ns2"}}}, true)
		dest := NewLokiDestination("d1", v1alpha1.ClusterLogDestinationSpec{})

		gen.AppendLogPipeline(source1, nil, []impl.LogDestination{dest})
		gen.AppendLogPipeline(source2, nil, []impl.LogDestination{dest})

		require.Len(t, gen.destinations, 1)
		assert.Equal(t, gen.destinations[0].(*lokiDestination).Inputs, []string{"d8_namespaced_source_ns1_s1", "d8_namespaced_source_ns2_s2"})
	})
}

func TestConfig_1(t *testing.T) {
	src := NewKubernetesLogSource("testsource", v1alpha1.KubernetesPodsSpec{
		NamespaceSelector: types.NameSelector{MatchNames: []string{"foot", "baar"}},
		LabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"aaaa": "bbbb"},
		},
	}, false)

	spec := v1alpha1.ClusterLogDestinationSpec{
		Loki: v1alpha1.LokiSpec{
			Endpoint: "http://testmeip:9000",
		},
	}

	dest := NewLokiDestination("testoutput", spec)

	gen := NewLogConfigGenerator()
	gen.AppendLogPipeline(src, nil, []impl.LogDestination{dest})

	conf, err := gen.GenerateConfig()
	require.NoError(t, err)

	assert.JSONEq(t, `
{
  "sources": {
    "d8_clusterns_source_baar_testsource": {
      "type": "kubernetes_logs",
      "extra_label_selector": "aaaa=bbbb",
      "extra_field_selector": "metadata.namespace=baar",
      "annotation_fields": {
        "container_image": "image",
        "container_name": "container",
        "pod_ip": "pod_ip",
        "pod_labels": "pod_labels",
        "pod_name": "pod",
        "pod_namespace": "namespace",
        "pod_node_name": "node",
        "pod_owner": "pod_owner"
      },
      "glob_minimum_cooldown_ms": 1000
    },
    "d8_clusterns_source_foot_testsource": {
      "type": "kubernetes_logs",
      "extra_label_selector": "aaaa=bbbb",
      "extra_field_selector": "metadata.namespace=foot",
      "annotation_fields": {
        "container_image": "image",
        "container_name": "container",
        "pod_ip": "pod_ip",
        "pod_labels": "pod_labels",
        "pod_name": "pod",
        "pod_namespace": "namespace",
        "pod_node_name": "node",
        "pod_owner": "pod_owner"
      },
      "glob_minimum_cooldown_ms": 1000
    }
  },
  "sinks": {
    "d8_cluster_sink_testoutput": {
      "type": "loki",
      "inputs": [
        "d8_clusterns_source_foot_testsource",
        "d8_clusterns_source_baar_testsource"
      ],
      "encoding": {
        "codec": "text",
        "timestamp_format": "rfc3339",
        "only_fields": ["message"]
      },
      "endpoint": "http://testmeip:9000",
      "buffer": {
        "max_size": 104857600,
        "type": "disk"
      },
	  "healthcheck": {
        "enabled": false
	  },
	  "labels": {
		"container": "{{ container }}",
		"image": "{{ image }}",
		"namespace": "{{ namespace }}",
		"node": "{{ node }}",
		"pod": "{{ pod }}",
		"pod_ip": "{{ pod_ip }}",
		"stream": "{{ stream }}",
		"pod_labels": "{{ pod_labels }}",
		"pod_owner": "{{ pod_owner }}"
	  },
	  "remove_label_fields": true,
	  "out_of_order_action": "rewrite_timestamp"
    }
  }
}
`, string(conf))
}

func TestConfig_2(t *testing.T) {
	src := NewKubernetesLogSource("testsource", v1alpha1.KubernetesPodsSpec{
		NamespaceSelector: types.NameSelector{MatchNames: []string{"foot", "baar"}},
		LabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"aaaa": "bbbb"},
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "baz",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"qux", "norf"},
			}},
		},
	}, false)

	spec := v1alpha1.ClusterLogDestinationSpec{
		Logstash: v1alpha1.LogstashSpec{
			Endpoint: "192.168.0.1:9000",
		},
	}

	spec.Logstash.TLS.CAFile = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMwRENDQWJpZ0F3SUJBZ0lVU21UcEpRRVNKcGwwbkNRUGtIcG9PL3dzbGhVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1CNFhEVEl3TURNeE5URXhNemN3TjFvWERUTXdNRE14TXpFeApNemN3TjFvd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBCk1JSUJDZ0tDQVFFQXpOanRYNWhrbFl6ZitPYng1MlBhcTBnSVA0Uy91MU9LaFMrengxeERHbHFQWXRqTDdwM2EKZUJGekRScHBYY3JhOFlXTDk3SnRuYVB6dmR1eW9FRWlUWXZrV3Jyd002c3pIOCtkR0gxTVRmQ1JHS1pRclhITApuVDZIUnY3cy9URmNKNkZnMlI1MDV2elBTK2J4V2d2ZmRaUjFjVG1BTHdkMllOZGUxcDR3UGZXKzg5TUp4WVgwCmRYck0vVk04OGNwUnNWUmxQNkh5TExzNTYyQm5Qc1dKWVRBZUxwbUkvTlcvYTN6YzFDemgwblBydU9vUTg0ZEUKVlRqYnVOTDB5SDNZajNPVy9LaGxJYlJuMXpvWVh4UHdRZ0tsMnhLZ0hIWHlEQUQvZnIzL0tOSDgrZ2grNlFNQQp1NnNQWTFYZjJHWENKa1hadVVSRzNtcGlkZll6empVLyt3SURBUUFCb3lBd0hqQUpCZ05WSFJNRUFqQUFNQkVHCkExVWRFUVFLTUFpQ0JuVmlkVzUwZFRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUljQk8yR3pYRU1ZbHU1MTAKRDIySlpxZHR5QUx1RVIrZkRwdHduS0hLZVJhd2lZTllOSldBVGVSWHNGMUlJTnhIWVJRY3llOEc4VE1oYk1Wawp2T2hWMER6RTFRdjRIWTJqU0o2bXlkaEFoUUtBUVNlSFZ2SG91Ny9BbDNGVDVPejkyaUZvcmU0QithRkZZeUk2CmF5S3RZdlcvTHBPdTFpMDdUeS9EVlkwVEI3LzBvYyt3bjN6UFRkV3ZjVUovS2ErU2lNSlh2ZnFoUmdEeCtBUVQKc25ZMkp6RkhTaVkvVjdVY2NBSGxaYVFPN3JzY3Y5Z2ZDRHREZy9BVTFSbUIrTDloM2NydTBraTE2SVN4TG82UApSbGMreGJNRmpKMGZoYnlySnQ4c0poUWtmenJIZjZJVXpmL3hpTm1QR2VrT2ovZVpHMWwwODlEckZMaE9wTTZSCnZ1a0pYUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
	spec.Logstash.TLS.CertFile = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMwRENDQWJpZ0F3SUJBZ0lVU21UcEpRRVNKcGwwbkNRUGtIcG9PL3dzbGhVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1CNFhEVEl3TURNeE5URXhNemN3TjFvWERUTXdNRE14TXpFeApNemN3TjFvd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBCk1JSUJDZ0tDQVFFQXpOanRYNWhrbFl6ZitPYng1MlBhcTBnSVA0Uy91MU9LaFMrengxeERHbHFQWXRqTDdwM2EKZUJGekRScHBYY3JhOFlXTDk3SnRuYVB6dmR1eW9FRWlUWXZrV3Jyd002c3pIOCtkR0gxTVRmQ1JHS1pRclhITApuVDZIUnY3cy9URmNKNkZnMlI1MDV2elBTK2J4V2d2ZmRaUjFjVG1BTHdkMllOZGUxcDR3UGZXKzg5TUp4WVgwCmRYck0vVk04OGNwUnNWUmxQNkh5TExzNTYyQm5Qc1dKWVRBZUxwbUkvTlcvYTN6YzFDemgwblBydU9vUTg0ZEUKVlRqYnVOTDB5SDNZajNPVy9LaGxJYlJuMXpvWVh4UHdRZ0tsMnhLZ0hIWHlEQUQvZnIzL0tOSDgrZ2grNlFNQQp1NnNQWTFYZjJHWENKa1hadVVSRzNtcGlkZll6empVLyt3SURBUUFCb3lBd0hqQUpCZ05WSFJNRUFqQUFNQkVHCkExVWRFUVFLTUFpQ0JuVmlkVzUwZFRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUljQk8yR3pYRU1ZbHU1MTAKRDIySlpxZHR5QUx1RVIrZkRwdHduS0hLZVJhd2lZTllOSldBVGVSWHNGMUlJTnhIWVJRY3llOEc4VE1oYk1Wawp2T2hWMER6RTFRdjRIWTJqU0o2bXlkaEFoUUtBUVNlSFZ2SG91Ny9BbDNGVDVPejkyaUZvcmU0QithRkZZeUk2CmF5S3RZdlcvTHBPdTFpMDdUeS9EVlkwVEI3LzBvYyt3bjN6UFRkV3ZjVUovS2ErU2lNSlh2ZnFoUmdEeCtBUVQKc25ZMkp6RkhTaVkvVjdVY2NBSGxaYVFPN3JzY3Y5Z2ZDRHREZy9BVTFSbUIrTDloM2NydTBraTE2SVN4TG82UApSbGMreGJNRmpKMGZoYnlySnQ4c0poUWtmenJIZjZJVXpmL3hpTm1QR2VrT2ovZVpHMWwwODlEckZMaE9wTTZSCnZ1a0pYUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
	spec.Logstash.TLS.KeyFile = "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRRE0yTzFmbUdTVmpOLzQKNXZIblk5cXJTQWcvaEwrN1U0cUZMN1BIWEVNYVdvOWkyTXZ1bmRwNEVYTU5HbWxkeXRyeGhZdjNzbTJkby9POQoyN0tnUVNKTmkrUmF1dkF6cXpNZno1MFlmVXhOOEpFWXBsQ3RjY3VkUG9kRy91ejlNVndub1dEWkhuVG0vTTlMCjV2RmFDOTkxbEhWeE9ZQXZCM1pnMTE3V25qQTk5Yjd6MHduRmhmUjFlc3o5VXp6eHlsR3hWR1Uvb2ZJc3V6bnIKWUdjK3hZbGhNQjR1bVlqODFiOXJmTnpVTE9IU2MrdTQ2aER6aDBSVk9OdTQwdlRJZmRpUGM1YjhxR1VodEdmWApPaGhmRS9CQ0FxWGJFcUFjZGZJTUFQOSt2ZjhvMGZ6NkNIN3BBd0M3cXc5alZkL1laY0ltUmRtNVJFYmVhbUoxCjlqUE9OVC83QWdNQkFBRUNnZ0VBZjJjTGUwRVVqYzZvSGUzRTFkek15MnBwZHRmaFIyaVY1bS9jcUVsQmtzcHcKRTFJeXc1MTVtdU4vWXM3aWFXc1k4TTNXVjUrcGZUblRCbW4xbHFYcjU3N3hyMXhFdUYzcDFnY1I5WUU0UytFcQozT0hUbTR3Q3p6RnNnVU5ic2IxWlMyeWJCMjVoMXFlMVpjZWtwQlJ1VG5xZThHU0t6TFVmY3V1QUdJc2FCZTRLCjBNd2Z1SnlIUnNqQkk1KzhnTlBUaCtqL2R1YkxQcE1aQ1cvd2d0d2ZUaTFQek9OTlBZWFNGODBBVm5PMnBmcGcKb3d2TnBIR2FkR1FQK2RVVkdMUUU4bHVCODkxQVI3ek5UTDR3OW5NeERERDJ4cmpwNEVJQnUwNjRHcFhYZW1pZQo1eUpuSGh4S1BMdXV5WnkxVmNBckhqbzRoZ1VrUmVZbUo1OWpXRXRyd1FLQmdRRC81bXlHVmZCaGR4Mm5uenBpCjhhLzVVNUE4M05UdGFIL3IvUFlGWm5GK2xuODlJNjd1NmNHcEVaMjJMZ2t5RGRFWnJ4cmp6QlUzWXpPdTloR0cKa3VPUXE4N1J2SXcyU2RTK2VERkZCSDErY1Q0bXRTUmFNc21mM3E0WXNna3Q4VDhYMXcybi80MUJTSHNkRHlTdAp0R2VjMTJzRjlJOGlkTzdsVllNdHRLNUJKd0tCZ1FETTdXYWNVK3d5cHdtSng4N0tCTkxyVDNmV3ljTzFzbWVzCmFoQXQrV2t1alUvMzhOSFJTZm8ybjU0dTBUbDJCbzBzWlVjMVN0S3Q1TkNrRzA1czk1Zklod25qSm4xdWtzRWgKakdKVHM1aW9samdYc2VxTkdtTUFKSzNHSWNqYUtNQTFSNHpkQ2VXa1hIMU5ZL0ovRFA3K2xEWEhkTzdDWFVIdQpzZm1wcG5rbkRRS0JnQ1M2TDQxQVBGWGd3TExVR1k4bDNQbk4wbi9KdWcydzE5dEkzUTU5VzRDdG5PbHJlNm55CmhzYjdMa1Y5YWZoekh6V3VlZytEdE8vVUh4RFhaRUNLU0hyMURhUHdpYmNvOVkyNHRtbVBjV3Q2V1U4NDVGVEYKd1VaZXNXSDkrMjlLbHFHWFRmQjByeE5Wa2NYajdJRzV5TDByOWNKUERWUUdzRnJkNFF2b1NMSTFBb0dCQUtHdwpTbjdiNUloT3JVTHR5T1l5aWl5cHhmZE51TUphNGx2eVQ1UEdyMHZRcWFFS2ZMSXlPVjd4OEFBbWlyenFER2RUCi9hdzV2aU1BWC9LcnJPUmpNbnBBdWZka3ZpRUpYNkxWdmhzbW9ET2NXdU92T0U1ZTNIQVhnSmpNdlVvTVR5TjYKc2RVUll3U2RDU3lQeUp5Z0oyMjhpUFkzOTg2WmdGVVNUZGVpaHdMZEFvR0FPakdkNDVSa1NlTWpKbmhKSmtqNQpHRWxrT2t6eCtBbzcyMFdDOUZoMHdPVTN4QUJycC9tWXF5cW9mdEpjSmlaeDkzYUlyL1Y5akhDMGpXdXV1U3FGCjF3K3BRd3M5VVd3WitLNExZZGMvTnp4SWRRMXRKYnR3Yzdia1RJOTZSbjFyZnZMc1I3K29LS01lUVNEb0dRV2EKR1pSRWdGYm1jdkhja2ZXZldkWHFURmM9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K"

	dest := NewLogstashDestination("testoutput", spec)

	gen := NewLogConfigGenerator()
	gen.AppendLogPipeline(src, nil, []impl.LogDestination{dest})

	conf, err := gen.GenerateConfig()
	require.NoError(t, err)

	assert.JSONEq(t, `
{
  "sources": {
    "d8_clusterns_source_baar_testsource": {
      "type": "kubernetes_logs",
      "extra_label_selector": "aaaa=bbbb,baz in (norf,qux)",
      "extra_field_selector": "metadata.namespace=baar",
      "annotation_fields": {
        "container_image": "image",
        "container_name": "container",
        "pod_ip": "pod_ip",
        "pod_labels": "pod_labels",
        "pod_name": "pod",
        "pod_namespace": "namespace",
        "pod_node_name": "node",
        "pod_owner": "pod_owner"
      },
      "glob_minimum_cooldown_ms": 1000
    },
    "d8_clusterns_source_foot_testsource": {
      "type": "kubernetes_logs",
      "extra_label_selector": "aaaa=bbbb,baz in (norf,qux)",
      "extra_field_selector": "metadata.namespace=foot",
      "annotation_fields": {
        "container_image": "image",
        "container_name": "container",
        "pod_ip": "pod_ip",
        "pod_labels": "pod_labels",
        "pod_name": "pod",
        "pod_namespace": "namespace",
        "pod_node_name": "node",
        "pod_owner": "pod_owner"
      },
      "glob_minimum_cooldown_ms": 1000
    }
  },
  "sinks": {
    "d8_cluster_sink_testoutput": {
      "type": "socket",
      "inputs": [
        "d8_clusterns_source_foot_testsource",
        "d8_clusterns_source_baar_testsource"
      ],
      "address": "192.168.0.1:9000",
      "mode": "tcp",
      "encoding": {
        "codec": "json",
        "timestamp_format": "rfc3339"
      },
      "healthcheck": {
        "enabled": false
      },
      "buffer": {
        "max_size": 104857600,
        "type": "disk"
      },
      "tls": {
          "ca_file": "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUSmTpJQESJpl0nCQPkHpoO/wslhUwDQYJKoZIhvcNAQEL\nBQAwETEPMA0GA1UEAwwGdWJ1bnR1MB4XDTIwMDMxNTExMzcwN1oXDTMwMDMxMzEx\nMzcwN1owETEPMA0GA1UEAwwGdWJ1bnR1MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\nMIIBCgKCAQEAzNjtX5hklYzf+Obx52Paq0gIP4S/u1OKhS+zx1xDGlqPYtjL7p3a\neBFzDRppXcra8YWL97JtnaPzvduyoEEiTYvkWrrwM6szH8+dGH1MTfCRGKZQrXHL\nnT6HRv7s/TFcJ6Fg2R505vzPS+bxWgvfdZR1cTmALwd2YNde1p4wPfW+89MJxYX0\ndXrM/VM88cpRsVRlP6HyLLs562BnPsWJYTAeLpmI/NW/a3zc1Czh0nPruOoQ84dE\nVTjbuNL0yH3Yj3OW/KhlIbRn1zoYXxPwQgKl2xKgHHXyDAD/fr3/KNH8+gh+6QMA\nu6sPY1Xf2GXCJkXZuURG3mpidfYzzjU/+wIDAQABoyAwHjAJBgNVHRMEAjAAMBEG\nA1UdEQQKMAiCBnVidW50dTANBgkqhkiG9w0BAQsFAAOCAQEAIcBO2GzXEMYlu510\nD22JZqdtyALuER+fDptwnKHKeRawiYNYNJWATeRXsF1IINxHYRQcye8G8TMhbMVk\nvOhV0DzE1Qv4HY2jSJ6mydhAhQKAQSeHVvHou7/Al3FT5Oz92iFore4B+aFFYyI6\nayKtYvW/LpOu1i07Ty/DVY0TB7/0oc+wn3zPTdWvcUJ/Ka+SiMJXvfqhRgDx+AQT\nsnY2JzFHSiY/V7UccAHlZaQO7rscv9gfCDtDg/AU1RmB+L9h3cru0ki16ISxLo6P\nRlc+xbMFjJ0fhbyrJt8sJhQkfzrHf6IUzf/xiNmPGekOj/eZG1l089DrFLhOpM6R\nvukJXQ==\n-----END CERTIFICATE-----\n",
          "crt_file": "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUSmTpJQESJpl0nCQPkHpoO/wslhUwDQYJKoZIhvcNAQEL\nBQAwETEPMA0GA1UEAwwGdWJ1bnR1MB4XDTIwMDMxNTExMzcwN1oXDTMwMDMxMzEx\nMzcwN1owETEPMA0GA1UEAwwGdWJ1bnR1MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\nMIIBCgKCAQEAzNjtX5hklYzf+Obx52Paq0gIP4S/u1OKhS+zx1xDGlqPYtjL7p3a\neBFzDRppXcra8YWL97JtnaPzvduyoEEiTYvkWrrwM6szH8+dGH1MTfCRGKZQrXHL\nnT6HRv7s/TFcJ6Fg2R505vzPS+bxWgvfdZR1cTmALwd2YNde1p4wPfW+89MJxYX0\ndXrM/VM88cpRsVRlP6HyLLs562BnPsWJYTAeLpmI/NW/a3zc1Czh0nPruOoQ84dE\nVTjbuNL0yH3Yj3OW/KhlIbRn1zoYXxPwQgKl2xKgHHXyDAD/fr3/KNH8+gh+6QMA\nu6sPY1Xf2GXCJkXZuURG3mpidfYzzjU/+wIDAQABoyAwHjAJBgNVHRMEAjAAMBEG\nA1UdEQQKMAiCBnVidW50dTANBgkqhkiG9w0BAQsFAAOCAQEAIcBO2GzXEMYlu510\nD22JZqdtyALuER+fDptwnKHKeRawiYNYNJWATeRXsF1IINxHYRQcye8G8TMhbMVk\nvOhV0DzE1Qv4HY2jSJ6mydhAhQKAQSeHVvHou7/Al3FT5Oz92iFore4B+aFFYyI6\nayKtYvW/LpOu1i07Ty/DVY0TB7/0oc+wn3zPTdWvcUJ/Ka+SiMJXvfqhRgDx+AQT\nsnY2JzFHSiY/V7UccAHlZaQO7rscv9gfCDtDg/AU1RmB+L9h3cru0ki16ISxLo6P\nRlc+xbMFjJ0fhbyrJt8sJhQkfzrHf6IUzf/xiNmPGekOj/eZG1l089DrFLhOpM6R\nvukJXQ==\n-----END CERTIFICATE-----\n",
          "key_file": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDM2O1fmGSVjN/4\n5vHnY9qrSAg/hL+7U4qFL7PHXEMaWo9i2Mvundp4EXMNGmldytrxhYv3sm2do/O9\n27KgQSJNi+RauvAzqzMfz50YfUxN8JEYplCtccudPodG/uz9MVwnoWDZHnTm/M9L\n5vFaC991lHVxOYAvB3Zg117WnjA99b7z0wnFhfR1esz9UzzxylGxVGU/ofIsuznr\nYGc+xYlhMB4umYj81b9rfNzULOHSc+u46hDzh0RVONu40vTIfdiPc5b8qGUhtGfX\nOhhfE/BCAqXbEqAcdfIMAP9+vf8o0fz6CH7pAwC7qw9jVd/YZcImRdm5REbeamJ1\n9jPONT/7AgMBAAECggEAf2cLe0EUjc6oHe3E1dzMy2ppdtfhR2iV5m/cqElBkspw\nE1Iyw515muN/Ys7iaWsY8M3WV5+pfTnTBmn1lqXr577xr1xEuF3p1gcR9YE4S+Eq\n3OHTm4wCzzFsgUNbsb1ZS2ybB25h1qe1ZcekpBRuTnqe8GSKzLUfcuuAGIsaBe4K\n0MwfuJyHRsjBI5+8gNPTh+j/dubLPpMZCW/wgtwfTi1PzONNPYXSF80AVnO2pfpg\nowvNpHGadGQP+dUVGLQE8luB891AR7zNTL4w9nMxDDD2xrjp4EIBu064GpXXemie\n5yJnHhxKPLuuyZy1VcArHjo4hgUkReYmJ59jWEtrwQKBgQD/5myGVfBhdx2nnzpi\n8a/5U5A83NTtaH/r/PYFZnF+ln89I67u6cGpEZ22LgkyDdEZrxrjzBU3YzOu9hGG\nkuOQq87RvIw2SdS+eDFFBH1+cT4mtSRaMsmf3q4Ysgkt8T8X1w2n/41BSHsdDySt\ntGec12sF9I8idO7lVYMttK5BJwKBgQDM7WacU+wypwmJx87KBNLrT3fWycO1smes\nahAt+WkujU/38NHRSfo2n54u0Tl2Bo0sZUc1StKt5NCkG05s95fIhwnjJn1uksEh\njGJTs5ioljgXseqNGmMAJK3GIcjaKMA1R4zdCeWkXH1NY/J/DP7+lDXHdO7CXUHu\nsfmppnknDQKBgCS6L41APFXgwLLUGY8l3PnN0n/Jug2w19tI3Q59W4CtnOlre6ny\nhsb7LkV9afhzHzWueg+DtO/UHxDXZECKSHr1DaPwibco9Y24tmmPcWt6WU845FTF\nwUZesWH9+29KlqGXTfB0rxNVkcXj7IG5yL0r9cJPDVQGsFrd4QvoSLI1AoGBAKGw\nSn7b5IhOrULtyOYyiiypxfdNuMJa4lvyT5PGr0vQqaEKfLIyOV7x8AAmirzqDGdT\n/aw5viMAX/KrrORjMnpAufdkviEJX6LVvhsmoDOcWuOvOE5e3HAXgJjMvUoMTyN6\nsdURYwSdCSyPyJygJ228iPY3986ZgFUSTdeihwLdAoGAOjGd45RkSeMjJnhJJkj5\nGElkOkzx+Ao720WC9Fh0wOU3xABrp/mYqyqoftJcJiZx93aIr/V9jHC0jWuuuSqF\n1w+pQws9UWwZ+K4LYdc/NzxIdQ1tJbtwc7bkTI96Rn1rfvLsR7+oKKMeQSDoGQWa\nGZREgFbmcvHckfWfWdXqTFc=\n-----END PRIVATE KEY-----\n",
          "enabled": true,
          "verify_certificate": false,
          "verify_hostname": false
      }
    }
  }
}
`, string(conf))
}

func TestConfig_3(t *testing.T) {
	src := NewFileLogSource("testfile", v1alpha1.FileSpec{
		Include: []string{"/var/log/*log", "/var/log/nginx/*.access.log"},
		Exclude: []string{"/var/log/syslog"},
	})

	spec := v1alpha1.ClusterLogDestinationSpec{
		Elasticsearch: v1alpha1.ElasticsearchSpec{
			Endpoint: "https://192.168.0.1:9200",
			Index:    "{{ kubernetes.namespace }}-%F",
			Pipeline: "test-pipe",
			TLS:      v1alpha1.CommonTLSSpec{VerifyHostname: true},
		},
	}

	dest := NewElasticsearchDestination("testoutput", spec)

	gen := NewLogConfigGenerator()
	gen.AppendLogPipeline(src, nil, []impl.LogDestination{dest})

	conf, err := gen.GenerateConfig()
	require.NoError(t, err)

	assert.JSONEq(t, `
{
  "sources": {
    "testfile": {
      "type": "file",
      "include": [
		  "/var/log/*log",
		  "/var/log/nginx/*.access.log"
	   ],
	  "exclude": [ "/var/log/syslog" ]
    }
  },
  "sinks": {
    "d8_cluster_sink_testoutput": {
      "type": "elasticsearch",
      "inputs": [
        "testfile"
      ],
      "endpoint": "https://192.168.0.1:9200",
      "encoding": {
        "timestamp_format": "rfc3339"
      },
      "tls": {
        "verify_hostname": true
      },
      "buffer": {
        "max_size": 104857600,
        "type": "disk"
      },
      "batch": {
          "timeout_secs": 1,
          "max_bytes": 10485760
      },
      "healthcheck": {
        "enabled": false
      },
	  "compression": "gzip",
    "bulk_action": "index",
    "mode": "normal",
    "index": "{{ kubernetes.namespace }}-%F",
    "pipeline": "test-pipe"
    }
  }
}
`, string(conf))
}
