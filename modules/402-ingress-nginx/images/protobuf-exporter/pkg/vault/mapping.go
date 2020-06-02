package vault

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v3"
)

type MappingType string

const (
	HistogramMapping = "Histogram"
	CounterMapping   = "Counter"
	GaugeMapping     = "Gauge"
)

type Mapping struct {
	Type MappingType `yaml:"type"`
	Name string      `yaml:"name"`
	Help string      `yaml:"help,omitempty"`

	LabelNames []string      `yaml:"labels,omitempty"`
	Buckets    []float64     `yaml:"buckets,omitempty"`
	TTL        time.Duration `yaml:"ttl,omitempty"`
}

func LoadMappings(fileContent []byte) ([]Mapping, error) {
	var mappings []Mapping
	err := yaml.Unmarshal(fileContent, &mappings)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling mappings: %v", err)
	}

	return mappings, nil
}

func LoadMappingsByPath(path string) ([]Mapping, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading mappings: %v", err)
	}
	return LoadMappings(fileContent)
}
