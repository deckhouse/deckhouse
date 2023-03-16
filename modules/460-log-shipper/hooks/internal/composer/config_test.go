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

package composer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/destination"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/source"
)

func compareMock(t *testing.T, data []byte, parts ...string) {
	filename := filepath.Join(append([]string{"testdata"}, parts...)...)
	content, err := os.ReadFile(filename)
	require.NoError(t, err)

	if os.Getenv("D8_LOG_SHIPPER_SAVE_TESTDATA") == "yes" {
		err := os.WriteFile(filename, data, 0600)
		require.NoError(t, err)
	}

	assert.JSONEq(t, string(content), string(data))
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name          string
		mockToCompare string
		buildFile     func() (apis.LogSource, apis.LogDestination)
	}{
		{
			name:          "Config 1",
			mockToCompare: "config_1.json",
			buildFile: func() (apis.LogSource, apis.LogDestination) {
				src := source.NewKubernetes("testsource", v1alpha1.KubernetesPodsSpec{
					NamespaceSelector: v1alpha1.NamespaceSelector{MatchNames: []string{"foot", "baar"}},
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"aaaa": "bbbb"},
					},
				}, false)

				spec := v1alpha1.ClusterLogDestinationSpec{
					Buffer: &v1alpha1.Buffer{
						Type: "Disk",
						Disk: v1alpha1.BufferDisk{
							MaxSize: *resource.NewQuantity(268435488, resource.BinarySI),
						},
						WhenFull: "Block",
					},
					Loki: v1alpha1.LokiSpec{
						Endpoint: "http://testmeip:9000",
					},
				}

				dest := destination.NewLoki("testoutput", spec)
				return src, dest
			},
		},
		{
			name:          "Config 2",
			mockToCompare: "config_2.json",
			buildFile: func() (apis.LogSource, apis.LogDestination) {
				src := source.NewKubernetes("testsource", v1alpha1.KubernetesPodsSpec{
					NamespaceSelector: v1alpha1.NamespaceSelector{MatchNames: []string{"foot", "baar"}},
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
					Buffer: &v1alpha1.Buffer{
						Type: "Disk",
						Disk: v1alpha1.BufferDisk{
							MaxSize: *resource.NewQuantity(215, resource.BinarySI),
						},
						WhenFull: "Block",
					},
				}

				spec.Logstash.TLS.CAFile = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMwRENDQWJpZ0F3SUJBZ0lVU21UcEpRRVNKcGwwbkNRUGtIcG9PL3dzbGhVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1CNFhEVEl3TURNeE5URXhNemN3TjFvWERUTXdNRE14TXpFeApNemN3TjFvd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBCk1JSUJDZ0tDQVFFQXpOanRYNWhrbFl6ZitPYng1MlBhcTBnSVA0Uy91MU9LaFMrengxeERHbHFQWXRqTDdwM2EKZUJGekRScHBYY3JhOFlXTDk3SnRuYVB6dmR1eW9FRWlUWXZrV3Jyd002c3pIOCtkR0gxTVRmQ1JHS1pRclhITApuVDZIUnY3cy9URmNKNkZnMlI1MDV2elBTK2J4V2d2ZmRaUjFjVG1BTHdkMllOZGUxcDR3UGZXKzg5TUp4WVgwCmRYck0vVk04OGNwUnNWUmxQNkh5TExzNTYyQm5Qc1dKWVRBZUxwbUkvTlcvYTN6YzFDemgwblBydU9vUTg0ZEUKVlRqYnVOTDB5SDNZajNPVy9LaGxJYlJuMXpvWVh4UHdRZ0tsMnhLZ0hIWHlEQUQvZnIzL0tOSDgrZ2grNlFNQQp1NnNQWTFYZjJHWENKa1hadVVSRzNtcGlkZll6empVLyt3SURBUUFCb3lBd0hqQUpCZ05WSFJNRUFqQUFNQkVHCkExVWRFUVFLTUFpQ0JuVmlkVzUwZFRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUljQk8yR3pYRU1ZbHU1MTAKRDIySlpxZHR5QUx1RVIrZkRwdHduS0hLZVJhd2lZTllOSldBVGVSWHNGMUlJTnhIWVJRY3llOEc4VE1oYk1Wawp2T2hWMER6RTFRdjRIWTJqU0o2bXlkaEFoUUtBUVNlSFZ2SG91Ny9BbDNGVDVPejkyaUZvcmU0QithRkZZeUk2CmF5S3RZdlcvTHBPdTFpMDdUeS9EVlkwVEI3LzBvYyt3bjN6UFRkV3ZjVUovS2ErU2lNSlh2ZnFoUmdEeCtBUVQKc25ZMkp6RkhTaVkvVjdVY2NBSGxaYVFPN3JzY3Y5Z2ZDRHREZy9BVTFSbUIrTDloM2NydTBraTE2SVN4TG82UApSbGMreGJNRmpKMGZoYnlySnQ4c0poUWtmenJIZjZJVXpmL3hpTm1QR2VrT2ovZVpHMWwwODlEckZMaE9wTTZSCnZ1a0pYUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
				spec.Logstash.TLS.CertFile = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMwRENDQWJpZ0F3SUJBZ0lVU21UcEpRRVNKcGwwbkNRUGtIcG9PL3dzbGhVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1CNFhEVEl3TURNeE5URXhNemN3TjFvWERUTXdNRE14TXpFeApNemN3TjFvd0VURVBNQTBHQTFVRUF3d0dkV0oxYm5SMU1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBCk1JSUJDZ0tDQVFFQXpOanRYNWhrbFl6ZitPYng1MlBhcTBnSVA0Uy91MU9LaFMrengxeERHbHFQWXRqTDdwM2EKZUJGekRScHBYY3JhOFlXTDk3SnRuYVB6dmR1eW9FRWlUWXZrV3Jyd002c3pIOCtkR0gxTVRmQ1JHS1pRclhITApuVDZIUnY3cy9URmNKNkZnMlI1MDV2elBTK2J4V2d2ZmRaUjFjVG1BTHdkMllOZGUxcDR3UGZXKzg5TUp4WVgwCmRYck0vVk04OGNwUnNWUmxQNkh5TExzNTYyQm5Qc1dKWVRBZUxwbUkvTlcvYTN6YzFDemgwblBydU9vUTg0ZEUKVlRqYnVOTDB5SDNZajNPVy9LaGxJYlJuMXpvWVh4UHdRZ0tsMnhLZ0hIWHlEQUQvZnIzL0tOSDgrZ2grNlFNQQp1NnNQWTFYZjJHWENKa1hadVVSRzNtcGlkZll6empVLyt3SURBUUFCb3lBd0hqQUpCZ05WSFJNRUFqQUFNQkVHCkExVWRFUVFLTUFpQ0JuVmlkVzUwZFRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUljQk8yR3pYRU1ZbHU1MTAKRDIySlpxZHR5QUx1RVIrZkRwdHduS0hLZVJhd2lZTllOSldBVGVSWHNGMUlJTnhIWVJRY3llOEc4VE1oYk1Wawp2T2hWMER6RTFRdjRIWTJqU0o2bXlkaEFoUUtBUVNlSFZ2SG91Ny9BbDNGVDVPejkyaUZvcmU0QithRkZZeUk2CmF5S3RZdlcvTHBPdTFpMDdUeS9EVlkwVEI3LzBvYyt3bjN6UFRkV3ZjVUovS2ErU2lNSlh2ZnFoUmdEeCtBUVQKc25ZMkp6RkhTaVkvVjdVY2NBSGxaYVFPN3JzY3Y5Z2ZDRHREZy9BVTFSbUIrTDloM2NydTBraTE2SVN4TG82UApSbGMreGJNRmpKMGZoYnlySnQ4c0poUWtmenJIZjZJVXpmL3hpTm1QR2VrT2ovZVpHMWwwODlEckZMaE9wTTZSCnZ1a0pYUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
				spec.Logstash.TLS.KeyFile = "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRRE0yTzFmbUdTVmpOLzQKNXZIblk5cXJTQWcvaEwrN1U0cUZMN1BIWEVNYVdvOWkyTXZ1bmRwNEVYTU5HbWxkeXRyeGhZdjNzbTJkby9POQoyN0tnUVNKTmkrUmF1dkF6cXpNZno1MFlmVXhOOEpFWXBsQ3RjY3VkUG9kRy91ejlNVndub1dEWkhuVG0vTTlMCjV2RmFDOTkxbEhWeE9ZQXZCM1pnMTE3V25qQTk5Yjd6MHduRmhmUjFlc3o5VXp6eHlsR3hWR1Uvb2ZJc3V6bnIKWUdjK3hZbGhNQjR1bVlqODFiOXJmTnpVTE9IU2MrdTQ2aER6aDBSVk9OdTQwdlRJZmRpUGM1YjhxR1VodEdmWApPaGhmRS9CQ0FxWGJFcUFjZGZJTUFQOSt2ZjhvMGZ6NkNIN3BBd0M3cXc5alZkL1laY0ltUmRtNVJFYmVhbUoxCjlqUE9OVC83QWdNQkFBRUNnZ0VBZjJjTGUwRVVqYzZvSGUzRTFkek15MnBwZHRmaFIyaVY1bS9jcUVsQmtzcHcKRTFJeXc1MTVtdU4vWXM3aWFXc1k4TTNXVjUrcGZUblRCbW4xbHFYcjU3N3hyMXhFdUYzcDFnY1I5WUU0UytFcQozT0hUbTR3Q3p6RnNnVU5ic2IxWlMyeWJCMjVoMXFlMVpjZWtwQlJ1VG5xZThHU0t6TFVmY3V1QUdJc2FCZTRLCjBNd2Z1SnlIUnNqQkk1KzhnTlBUaCtqL2R1YkxQcE1aQ1cvd2d0d2ZUaTFQek9OTlBZWFNGODBBVm5PMnBmcGcKb3d2TnBIR2FkR1FQK2RVVkdMUUU4bHVCODkxQVI3ek5UTDR3OW5NeERERDJ4cmpwNEVJQnUwNjRHcFhYZW1pZQo1eUpuSGh4S1BMdXV5WnkxVmNBckhqbzRoZ1VrUmVZbUo1OWpXRXRyd1FLQmdRRC81bXlHVmZCaGR4Mm5uenBpCjhhLzVVNUE4M05UdGFIL3IvUFlGWm5GK2xuODlJNjd1NmNHcEVaMjJMZ2t5RGRFWnJ4cmp6QlUzWXpPdTloR0cKa3VPUXE4N1J2SXcyU2RTK2VERkZCSDErY1Q0bXRTUmFNc21mM3E0WXNna3Q4VDhYMXcybi80MUJTSHNkRHlTdAp0R2VjMTJzRjlJOGlkTzdsVllNdHRLNUJKd0tCZ1FETTdXYWNVK3d5cHdtSng4N0tCTkxyVDNmV3ljTzFzbWVzCmFoQXQrV2t1alUvMzhOSFJTZm8ybjU0dTBUbDJCbzBzWlVjMVN0S3Q1TkNrRzA1czk1Zklod25qSm4xdWtzRWgKakdKVHM1aW9samdYc2VxTkdtTUFKSzNHSWNqYUtNQTFSNHpkQ2VXa1hIMU5ZL0ovRFA3K2xEWEhkTzdDWFVIdQpzZm1wcG5rbkRRS0JnQ1M2TDQxQVBGWGd3TExVR1k4bDNQbk4wbi9KdWcydzE5dEkzUTU5VzRDdG5PbHJlNm55CmhzYjdMa1Y5YWZoekh6V3VlZytEdE8vVUh4RFhaRUNLU0hyMURhUHdpYmNvOVkyNHRtbVBjV3Q2V1U4NDVGVEYKd1VaZXNXSDkrMjlLbHFHWFRmQjByeE5Wa2NYajdJRzV5TDByOWNKUERWUUdzRnJkNFF2b1NMSTFBb0dCQUtHdwpTbjdiNUloT3JVTHR5T1l5aWl5cHhmZE51TUphNGx2eVQ1UEdyMHZRcWFFS2ZMSXlPVjd4OEFBbWlyenFER2RUCi9hdzV2aU1BWC9LcnJPUmpNbnBBdWZka3ZpRUpYNkxWdmhzbW9ET2NXdU92T0U1ZTNIQVhnSmpNdlVvTVR5TjYKc2RVUll3U2RDU3lQeUp5Z0oyMjhpUFkzOTg2WmdGVVNUZGVpaHdMZEFvR0FPakdkNDVSa1NlTWpKbmhKSmtqNQpHRWxrT2t6eCtBbzcyMFdDOUZoMHdPVTN4QUJycC9tWXF5cW9mdEpjSmlaeDkzYUlyL1Y5akhDMGpXdXV1U3FGCjF3K3BRd3M5VVd3WitLNExZZGMvTnp4SWRRMXRKYnR3Yzdia1RJOTZSbjFyZnZMc1I3K29LS01lUVNEb0dRV2EKR1pSRWdGYm1jdkhja2ZXZldkWHFURmM9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K"

				dest := destination.NewLogstash("testoutput", spec)
				return src, dest
			},
		},
		{
			name:          "Config 3",
			mockToCompare: "config_3.json",
			buildFile: func() (apis.LogSource, apis.LogDestination) {
				src := source.NewFile("testfile", v1alpha1.FileSpec{
					Include: []string{"/var/log/*log", "/var/log/nginx/*.access.log"},
					Exclude: []string{"/var/log/syslog"},
				})

				spec := v1alpha1.ClusterLogDestinationSpec{
					Elasticsearch: v1alpha1.ElasticsearchSpec{
						Endpoint: "https://192.168.0.1:9200",
						Index:    "{{ kubernetes.namespace }}-%F",
						Pipeline: "test-pipe",
						TLS:      v1alpha1.CommonTLSSpec{VerifyHostname: pointer.Bool(true)},
					},
				}

				dest := destination.NewElasticsearch("testoutput", spec)
				return src, dest
			},
		},
		{
			name:          "Config 4",
			mockToCompare: "config_4.json",
			buildFile: func() (apis.LogSource, apis.LogDestination) {
				src := source.NewKubernetes("testsource", v1alpha1.KubernetesPodsSpec{}, false)

				spec := v1alpha1.ClusterLogDestinationSpec{
					Vector: v1alpha1.VectorSpec{
						Endpoint: "192.168.0.1:9200",
						TLS:      v1alpha1.CommonTLSSpec{VerifyCertificate: pointer.Bool(false)},
					},
				}

				dest := destination.NewVector("testoutput", spec)
				return src, dest
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, dest := tc.buildFile()

			file := NewVectorFile()
			err := file.AppendLogPipeline(&Pipeline{
				Source: PipelineSource{Source: src},
				Destinations: []PipelineDestination{
					{Destination: dest},
				},
			})
			require.NoError(t, err)

			conf, err := file.ConvertToJSON()
			require.NoError(t, err)

			compareMock(t, conf, tc.mockToCompare)
		})
	}
}
