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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
)

// Pipeline is a representation of a single logical tube.
//
//	Example: ClusterLoggingConfig +(destinationRef) ClusterLogsDestination = Single Pipeline.
type Pipeline struct {
	Source       PipelineSource
	Destinations []PipelineDestination
}

type PipelineSource struct {
	Source     apis.LogSource
	Transforms []apis.LogTransform
}

type PipelineDestination struct {
	Inputs set.Set

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

// AppendLogPipeline adds the pipeline to the accumulated ones.
// Pipeline always contains a single log source and one or more transform rules / sinks.
func (v *VectorFile) AppendLogPipeline(pipeline *Pipeline) error {
	compiledSrc := pipeline.Source.Source.BuildSources()
	for _, src := range compiledSrc {
		v.Sources[src.GetName()] = src
	}

	sourcesNames := set.New()
	for _, src := range compiledSrc {
		sourcesNames.Add(src.GetName())
	}

	destinationInputs := make([]string, 0)

	// If source has attached transforms, use the first one for all generated source
	// and the last one for the destination input
	if len(pipeline.Source.Transforms) > 0 {
		pipeline.Source.Transforms[0].SetInputs(sourcesNames.Slice())

		lastTransform := pipeline.Source.Transforms[len(pipeline.Source.Transforms)-1].GetName()
		destinationInputs = append(destinationInputs, lastTransform)
	} else {
		destinationInputs = append(destinationInputs, sourcesNames.Slice()...)
	}

	for _, trans := range pipeline.Source.Transforms {
		v.Transforms[trans.GetName()] = trans
	}

	for _, pipelineDest := range pipeline.Destinations {
		dest := pipelineDest.Destination

		if _, ok := v.Sinks[dest.GetName()]; !ok {
			v.Sinks[dest.GetName()] = dest
		}

		for _, trans := range pipelineDest.Transforms {
			if _, ok := v.Transforms[trans.GetName()]; !ok {
				v.Transforms[trans.GetName()] = trans
			}
		}

		if len(pipelineDest.Transforms) > 0 {
			v.Transforms[pipelineDest.Transforms[0].GetName()].SetInputs(destinationInputs)

			v.Sinks[dest.GetName()].SetInputs([]string{
				pipelineDest.Transforms[len(pipelineDest.Transforms)-1].GetName(),
			})
		} else {
			v.Sinks[dest.GetName()].SetInputs(destinationInputs)
		}
	}

	return nil
}
