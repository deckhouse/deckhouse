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

package transformation

import (
	"fmt"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func ReplaceKeysVRL(r v1alpha1.ReplaceKeysSpec) (string, error) {
	if r.Source == "" {
		return "", fmt.Errorf("transformations replaceKeys: Source is empty")
	}
	paths, err := parser.MapLabelPaths(r.Labels, parser.PathSegmentsToVRLDotPath)
	if err != nil {
		return "", fmt.Errorf("transformations replaceKeys: %w", err)
	}
	spec := struct {
		Source string
		Target string
		Paths  []string
	}{Source: r.Source, Target: r.Target, Paths: paths}
	source, err := vrl.ReplaceKeys.Render(vrl.Args{"spec": spec})
	if err != nil {
		return "", fmt.Errorf("transformations replaceKeys render error: %v", err)
	}
	return source, nil
}
