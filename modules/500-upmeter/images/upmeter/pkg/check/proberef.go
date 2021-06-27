/*
Copyright 2021 Flant CJSC

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

package check

import (
	"fmt"
	"os"
	"strings"
)

type ProbeRef struct {
	Group string `json:"group"`
	Probe string `json:"probe"`
}

func (p ProbeRef) Id() string {
	return fmt.Sprintf("%s/%s", p.Group, p.Probe)
}

var (
	enabledProbesList  []string
	disabledProbesList []string
)

func IsProbeEnabled(probeId string) bool {
	if enabledProbesList == nil {
		enabledProbesList = loadListFromString(os.Getenv("UPMETER_ENABLED_PROBES"))
	}
	if disabledProbesList == nil {
		disabledProbesList = loadListFromString(os.Getenv("UPMETER_DISABLED_PROBES"))
	}

	enabled := true

	if len(enabledProbesList) > 0 {
		enabled = false
		for _, enabledPrefix := range enabledProbesList {
			if !strings.Contains(enabledPrefix, "/") {
				enabledPrefix += "/"
			}
			if strings.HasPrefix(probeId, enabledPrefix) {
				enabled = true
				break
			}
		}
	}

	if enabled && len(disabledProbesList) > 0 {
		for _, disabledPrefix := range disabledProbesList {
			if !strings.Contains(disabledPrefix, "/") {
				disabledPrefix += "/"
			}
			if strings.HasPrefix(probeId, disabledPrefix) {
				enabled = false
				break
			}
		}
	}

	return enabled
}

// loadListFromString split environment variable by commas and return only non-empty parts.
func loadListFromString(input string) []string {
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	res := make([]string, 0)
	for _, part := range parts {
		if part != "" {
			res = append(res, part)
		}
	}

	return res
}
