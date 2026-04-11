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
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

var keepChildKeysKeyRe = regexp.MustCompile(`^[a-zA-Z0-9_./:@%#*+\-]+$`)

func isKeepChildKeysKey(s string) bool {
	return keepChildKeysKeyRe.MatchString(s)
}

func DropLabelsVRL(d v1alpha1.DropLabelsSpec) (string, []string, error) {
	if len(d.Labels) == 0 {
		return "", nil, fmt.Errorf("dropLabels: labels is empty")
	}
	if len(d.KeepChildKeys) == 0 {
		paths, err := parser.MapLabelPaths(d.Labels, parser.PathSegmentsToVRLDotPath)
		if err != nil {
			return "", nil, fmt.Errorf("dropLabels: %w", err)
		}
		s, err := vrl.DropLabels.Render(vrl.Args{"spec": struct {
			Paths []string
		}{Paths: paths}})
		return s, paths, err
	}
	for _, k := range d.KeepChildKeys {
		if !isKeepChildKeysKey(k) {
			return "", nil, fmt.Errorf("dropLabels: invalid keepChildKeys key %q", k)
		}
	}
	pathArrays, err := parser.MapLabelPaths(d.Labels, parser.PathSegmentsToVRLArray)
	if err != nil {
		return "", nil, fmt.Errorf("dropLabels: %w", err)
	}
	parts := make([]string, 0, len(pathArrays))
	for _, pa := range pathArrays {
		s, err := vrl.DropLabelsKeepChildKeys.Render(vrl.Args{"pathArray": pa, "keepKeys": d.KeepChildKeys})
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n"), nil, nil
}
