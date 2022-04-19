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

package config

import (
	"bytes"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transform"
)

// VectorFile is a vector config file corresponding golang structure.
type VectorFile struct {
	Sources    map[string]impl.LogSource      `json:"sources,omitempty"`
	Transforms map[string]impl.LogTransform   `json:"transforms,omitempty"`
	Sinks      map[string]impl.LogDestination `json:"sinks,omitempty"`
}

// NewVectorFile inits a VectorFile instance.
func NewVectorFile() *VectorFile {
	return &VectorFile{
		Sources:    make(map[string]impl.LogSource),
		Transforms: make(map[string]impl.LogTransform),
		Sinks:      make(map[string]impl.LogDestination),
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
	sources         []impl.LogSource
	transformations []impl.LogTransform
	destinations    []impl.LogDestination
}

// NewLogConfigGenerator return a new instance of a LogConfigGenerator.
func NewLogConfigGenerator() *LogConfigGenerator {
	return &LogConfigGenerator{}
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

	currentDest := g.destMap()

	for _, dest := range pipeline.Destinations {
		if cDest, ok := currentDest[dest.GetName()]; ok {
			cDest.AppendInputs(destinationInputs)
			continue
		}

		dest.AppendInputs(destinationInputs)
		g.destinations = append(g.destinations, dest)
	}

	g.sources = append(g.sources, sources...)
	g.transformations = append(g.transformations, pipeline.Transforms...)
}

// BuildTransforms returns a formatted ordered list off transform rules that should be applied to the pipelines.
func (g *LogConfigGenerator) BuildTransforms(name string, trans []impl.LogTransform) ([]impl.LogTransform, error) {
	return transform.BuildFromMapSlice(name, trans)
}

func (g *LogConfigGenerator) destMap() map[string]impl.LogDestination {
	m := make(map[string]impl.LogDestination, len(g.destinations))
	for _, d := range g.destinations {
		m[d.GetName()] = d
	}

	return m
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
