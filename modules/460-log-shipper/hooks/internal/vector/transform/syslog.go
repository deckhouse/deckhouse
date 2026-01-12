/*
Copyright 2023 Flant JSC

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

package transform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func SyslogEncoding() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "syslog_encoding",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.SyslogEncodingRule.String(),
			"drop_on_abort": false,
		},
	}
}

func SyslogExtraLabelsTransform(extraLabels map[string]string) *DynamicTransform {
	// Build VRL code to collect extraLabels into .syslog.extra_labels map
	vrlLines := []string{
		"if !exists(.syslog) { .syslog = {} }",
		"if !exists(.syslog.extra_labels) { .syslog.extra_labels = {} }",
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(extraLabels))
	for k := range extraLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := extraLabels[key]
		escapedKey := strings.ReplaceAll(key, `"`, `\"`)
		escapedValue := strings.ReplaceAll(value, `"`, `\"`)
		vrlLines = append(vrlLines, fmt.Sprintf(`.syslog.extra_labels."%s" = "%s"`, escapedKey, escapedValue))
	}

	vrlCode := strings.Join(vrlLines, "\n")

	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "syslog_extra_labels",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrlCode,
			"drop_on_abort": false,
		},
	}
}
