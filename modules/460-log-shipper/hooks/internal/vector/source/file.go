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

package source

import (
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

var _ apis.LogSource = (*File)(nil)

// File represents `file` vector source
// https://vector.dev/docs/reference/configuration/sources/file/
type File struct {
	commonSource

	Exclude   []string `json:"exclude,omitempty"`
	Include   []string `json:"include,omitempty"`
	Delimiter string   `json:"line_delimiter,omitempty"`
}

func NewFile(name string, spec v1alpha1.FileSpec) *File {
	return &File{
		commonSource: commonSource{
			Name: "cluster_logging_config/" + name,
			Type: "file",
		},
		Exclude:   spec.Exclude,
		Include:   spec.Include,
		Delimiter: spec.LineDelimiter,
	}
}

func (f *File) BuildSources() []apis.LogSource {
	return []apis.LogSource{f}
}
