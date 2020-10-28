package types

import (
	"fmt"
	"os"
	"strings"
)

type ProbeRef struct {
	Group string `json:"group"`
	Probe string `json:"probe"`
}

func (p ProbeRef) ProbeId() string {
	return fmt.Sprintf("%s/%s", p.Group, p.Probe)
}

var enabledProbesList []string
var disabledProbesList []string

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
