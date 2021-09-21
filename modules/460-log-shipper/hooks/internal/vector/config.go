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
	"bytes"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
)

type LogConfigGenerator struct {
	sources         []impl.LogSource
	transformations []impl.LogTransform
	destinations    []impl.LogDestination
}

func NewLogConfigGenerator() *LogConfigGenerator {
	return &LogConfigGenerator{}
}

func (vcg *LogConfigGenerator) AppendLogPipeline(inputSource impl.LogSource, transforms []impl.LogTransform, destinations []impl.LogDestination) {
	sources := inputSource.BuildSources()

	sourcesNames := make([]string, 0, len(sources))
	for _, source := range sources {
		sourcesNames = append(sourcesNames, source.GetName())
	}

	destinationInputs := make([]string, 0)
	if len(transforms) > 0 {
		transforms[0].SetInputs(sourcesNames)
		destinationInputs = append(destinationInputs, transforms[len(transforms)-1].GetName())
	} else {
		destinationInputs = sourcesNames
	}

	currentDest := vcg.destinationsMap()

	for _, dest := range destinations {
		if cdest, ok := currentDest[dest.GetName()]; ok {
			cdest.AppendInputs(destinationInputs)
			continue
		}

		dest.AppendInputs(destinationInputs)
		vcg.destinations = append(vcg.destinations, dest)
	}

	vcg.sources = append(vcg.sources, sources...)
	vcg.transformations = append(vcg.transformations, transforms...)
}

func (vcg *LogConfigGenerator) BuildTransformsFromMapSlice(inputName string, trans []impl.LogTransform) ([]impl.LogTransform, error) {
	return BuildTransformsFromMapSlice(inputName, trans)
}

func (vcg *LogConfigGenerator) destinationsMap() map[string]impl.LogDestination {
	m := make(map[string]impl.LogDestination, len(vcg.destinations))
	for _, d := range vcg.destinations {
		m[d.GetName()] = d
	}

	return m
}

type vectorConfig struct {
	Sources    map[string]impl.LogSource      `json:"sources,omitempty"`
	Transforms map[string]impl.LogTransform   `json:"transforms,omitempty"`
	Sinks      map[string]impl.LogDestination `json:"sinks,omitempty"`
}

func (vcg *LogConfigGenerator) GenerateConfig() ([]byte, error) {
	sourcesMap := make(map[string]impl.LogSource, len(vcg.sources))
	transMap := make(map[string]impl.LogTransform, len(vcg.transformations))
	destMap := make(map[string]impl.LogDestination, len(vcg.destinations))

	for _, src := range vcg.sources {
		sourcesMap[src.GetName()] = src
	}

	for _, tr := range vcg.transformations {
		transMap[tr.GetName()] = tr
	}

	for _, dest := range vcg.destinations {
		destMap[dest.GetName()] = dest
	}

	result := vectorConfig{
		Sources:    sourcesMap,
		Transforms: transMap,
		Sinks:      destMap,
	}

	buf := bytes.NewBuffer(nil)

	en := json.NewEncoder(buf)
	en.SetIndent("", "  ")

	err := en.Encode(result)

	return buf.Bytes(), err
}
