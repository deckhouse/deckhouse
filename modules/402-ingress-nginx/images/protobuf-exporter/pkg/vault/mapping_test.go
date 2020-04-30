package vault

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadMappings(t *testing.T) {
	tests := []struct {
		name             string
		content          []byte
		expectedMappings []Mapping
		wantErr          bool
	}{
		{
			name: "EveryMappings",
			content: []byte(`
- name: test_counter
  type: Counter
  labels: ["server", "location"]
  ttl: 1h
- name: test_histogram
  type: Histogram
  labels: ["server", "location"]
  buckets: [0, 1, 2]
  ttl: 5m
- name: test_gauge
  type: Gauge
  help: useful metric
`),
			expectedMappings: []Mapping{
				{Name: "test_counter", Type: CounterMapping, LabelNames: []string{"server", "location"}, TTL: time.Hour},
				{Name: "test_histogram", Type: HistogramMapping, LabelNames: []string{"server", "location"}, TTL: 5 * time.Minute, Buckets: []float64{0, 1, 2}},
				{Name: "test_gauge", Type: GaugeMapping, Help: "useful metric"},
			},
		},
		{
			name:    "With error",
			wantErr: true,
			content: []byte(`
- name: test_counter
  type: Wrong YAML:
`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mappings, err := LoadMappings(tc.content)
			if err != nil && !tc.wantErr {
				t.Fatalf("load mappings error: %v", err)
			}
			if tc.wantErr && err == nil {
				t.Fatalf("load mappings error: %v", err)
			}

			if len(mappings) != len(tc.expectedMappings) {
				t.Fatalf("receive mappings count %v, expected %v", len(mappings), len(tc.expectedMappings))
			}

			for i := 0; i < len(mappings); i++ {
				if !reflect.DeepEqual(mappings[i], tc.expectedMappings[i]) {
					t.Fatalf("mappings differ: \n%v\n\n%v", mappings[i], tc.expectedMappings[i])
				}
			}
		})
	}
}
