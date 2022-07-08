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
	"bytes"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
)

// Pipeline is a representation of a single logical tube.
//   Example: ClusterLoggingConfig +(destinationRef) ClusterLogsDestination = Single Pipeline.
type Pipeline struct {
	Source       apis.LogSource
	Transforms   []apis.LogTransform
	Destinations []PipelineDestination
}

type PipelineDestination struct {
	Destination apis.LogDestination
	Transforms  []apis.LogTransform
}

// VectorFile is a vector config file corresponding golang structure.
type VectorFile struct {
	Sources    map[string]apis.LogSource      `json:"sources,omitempty"`
	Transforms map[string]apis.LogTransform   `json:"transforms,omitempty"`
	Sinks      map[string]apis.LogDestination `json:"sinks,omitempty"`
}

// NewVectorFile inits a VectorFile instance.
func NewVectorFile() *VectorFile {
	return &VectorFile{
		Sources:    make(map[string]apis.LogSource),
		Transforms: make(map[string]apis.LogTransform),
		Sinks:      make(map[string]apis.LogDestination),
	}
}

func (v *VectorFile) empty() bool {
	return len(v.Sources)+len(v.Sinks)+len(v.Transforms) == 0
}

// ConvertToJSON converts the vector file to the pretty-formatted JSON document.
func (v *VectorFile) ConvertToJSON() ([]byte, error) {
	if v.empty() {
		return nil, nil
	}

	buf := bytes.NewBuffer(nil)

	en := json.NewEncoder(buf)
	en.SetIndent("", "  ")

	err := en.Encode(v)

	return buf.Bytes(), err
}

// LogConfigGenerator accumulates pipelines and converts them to the vector config file.
type LogConfigGenerator struct {
	sources         []apis.LogSource
	transformations []apis.LogTransform
	destinations    map[string]apis.LogDestination
}

// NewLogConfigGenerator return a new instance of a LogConfigGenerator.
func NewLogConfigGenerator() *LogConfigGenerator {
	return &LogConfigGenerator{
		destinations: make(map[string]apis.LogDestination),
	}
}

// AppendLogPipeline adds the pipeline to the accumulated ones.
// Pipeline always contains a single log source and one or more transform rules / sinks.
func (g *LogConfigGenerator) AppendLogPipeline(pipeline *Pipeline) {
	sources := pipeline.Source.BuildSources()

	sourcesNames := make([]string, 0, len(sources))
	for _, source := range sources {
		sourcesNames = append(sourcesNames, source.GetName())
	}

	destinationInputs := make([]string, 0)

	if len(pipeline.Transforms) > 0 {
		pipeline.Transforms[0].SetInputs(sourcesNames)
		destinationInputs = append(destinationInputs, pipeline.Transforms[len(pipeline.Transforms)-1].GetName())
	} else {
		destinationInputs = sourcesNames
	}

	for _, pipelineDest := range pipeline.Destinations {
		dest := pipelineDest.Destination

		if _, ok := g.destinations[dest.GetName()]; !ok {
			g.destinations[dest.GetName()] = dest
		}

		g.destinations[dest.GetName()].AppendInputs(destinationInputs)
	}

	g.sources = append(g.sources, sources...)
	g.transformations = append(g.transformations, pipeline.Transforms...)
}

// GenerateConfig returns collected pipelines as a JSON formatted document.
func (g *LogConfigGenerator) GenerateConfig() ([]byte, error) {
	vectorFile := NewVectorFile()

	for _, src := range g.sources {
		vectorFile.Sources[src.GetName()] = src
	}

	for _, tr := range g.transformations {
		vectorFile.Transforms[tr.GetName()] = tr
	}

	for _, dest := range g.destinations {
		vectorFile.Sinks[dest.GetName()] = dest
	}

	return vectorFile.ConvertToJSON()
}
