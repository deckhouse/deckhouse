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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func SyslogK8sLabelsTransform() *DynamicTransform {
	k8sLabels := []string{
		"namespace",
		"container",
		"image",
		"pod",
		"node",
		"pod_ip",
		"stream",
		"node_group",
		"pod_owner",
		"host",
	}

	rule, err := vrl.SyslogK8sLabelsRule.Render(vrl.Args{
		"k8sLabels": k8sLabels,
	})
	if err != nil {
		return nil
	}

	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "syslog_k8s_labels",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        rule,
			"drop_on_abort": false,
		},
	}
}

func SyslogExtraLabelsTransform(extraLabels map[string]string) *DynamicTransform {
	keys := make([]string, 0, len(extraLabels))
	for k := range extraLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make(map[string]string)
	for _, k := range keys {
		escapedKey := escapeVectorString(k)
		fields[k] = fmt.Sprintf(".%s", escapedKey)
	}

	rule, err := vrl.SyslogExtraLabelsRule.Render(vrl.Args{
		"extraLabels": fields,
	})
	if err != nil {
		return nil
	}

	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "syslog_extra_labels",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        rule,
			"drop_on_abort": false,
		},
	}
}

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
