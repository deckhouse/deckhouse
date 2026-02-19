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
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

// SyslogLabelsTransform builds one remap that sets .sd_labels [name="value" ...] for syslog RFC 5424.
// Uses loglabels.GetSyslogLabels (mergeLabels + sorted keys, like other destinations).
func SyslogLabelsTransform(sourceType string, extraLabels map[string]string) *DynamicTransform {
	labels := loglabels.GetSyslogLabels(sourceType, extraLabels)

	rule, err := vrl.SyslogLabelsRule.Render(vrl.Args{
		"labels": labels,
	})
	if err != nil {
		return nil
	}

	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "syslog_labels",
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
