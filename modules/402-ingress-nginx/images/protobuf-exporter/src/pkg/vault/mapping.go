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
