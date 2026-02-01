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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

// SyslogLabelsTransform builds one remap that sets .k8s_labels and .extra_labels for syslog structured-data (RFC 5424).
// Labels depend on the pipeline source type (KubernetesPods vs File), same as Loki/CEF/Splunk destinations.
func SyslogLabelsTransform(sourceType string, extraLabels map[string]string) *DynamicTransform {
	sourceKeys, extraKeys := loglabels.GetSyslogLabels(sourceType, extraLabels)

	extraFields := make(map[string]string)
	for _, k := range extraKeys {
		escapedKey := escapeVectorString(k)
		extraFields[k] = fmt.Sprintf(".%s", escapedKey)
	}

	rule, err := vrl.SyslogLabelsRule.Render(vrl.Args{
		"sourceLabels": sourceKeys,
		"extraLabels":  extraFields,
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
