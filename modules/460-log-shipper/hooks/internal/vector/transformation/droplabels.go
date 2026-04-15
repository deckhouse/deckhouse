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

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

var keepKeysKeyRe = regexp.MustCompile(`^[a-zA-Z0-9_./:@%#*+\-]+$`)

func isKeepKeysKey(s string) bool {
	return keepKeysKeyRe.MatchString(s)
}

func DropLabelsVRL(d v1alpha2.DropLabelsSpec) (string, []string, error) {
	if len(d.Labels) == 0 {
		return "", nil, fmt.Errorf("dropLabels: labels is empty")
	}
	var dropPaths []string
	var parts []string
	for _, item := range d.Labels {
		path := strings.TrimSpace(item.Label)
		if path == "" {
			return "", nil, fmt.Errorf("dropLabels: empty label path")
		}
		if len(item.KeepKeys) == 0 {
			dropPaths = append(dropPaths, path)
			continue
		}
		for _, k := range item.KeepKeys {
			if !isKeepKeysKey(k) {
				return "", nil, fmt.Errorf("dropLabels: invalid keepKeys entry %q", k)
			}
		}
		segs, err := parser.ParseLabelPath(path)
		if err != nil {
			return "", nil, fmt.Errorf("dropLabels: %w", err)
		}
		pa := parser.PathSegmentsToVRLArray(segs)
		s, err := vrl.DropLabelsKeepKeys.Render(vrl.Args{"pathArray": pa, "keepKeys": item.KeepKeys})
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, s)
	}
	if len(dropPaths) > 0 {
		paths, err := parser.MapLabelPaths(dropPaths, parser.PathSegmentsToVRLDotPath)
		if err != nil {
			return "", nil, fmt.Errorf("dropLabels: %w", err)
		}
		s, err := vrl.DropLabels.Render(vrl.Args{"spec": struct {
			Paths []string
		}{Paths: paths}})
		if err != nil {
			return "", nil, err
		}
		parts = append([]string{s}, parts...)
		return strings.Join(parts, "\n"), dropPaths, nil
	}
	return strings.Join(parts, "\n"), nil, nil
}
