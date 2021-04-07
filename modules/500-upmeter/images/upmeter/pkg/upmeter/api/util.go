package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type timerange struct {
	from, to, step int64
}

// DecodeFromToStep decodes 3 arguments
func DecodeFromToStep(fromArg, toArg, stepArg string) (timerange, error) {
	var (
		hasFrom = fromArg != ""
		hasTo   = toArg != ""
		hasStep = stepArg != ""
		err     error
	)
	r := timerange{step: 30}

	if hasFrom {
		r.from, err = parseTimestamp(fromArg)
		if err != nil {
			return r, fmt.Errorf("from=%q is not timestamp: %v", fromArg, err)
		}
	}

	if hasTo {
		r.to, err = parseTimestamp(toArg)
		if err != nil {
			return r, fmt.Errorf("to=%q is not timestamp: %v", toArg, err)
		}
	}

	if hasStep {
		r.step, err = parseDuration(stepArg)
		if err != nil {
			return r, fmt.Errorf("step=%q is not duration: %v", stepArg, err)
		}
	}

	// "from-to" variant
	if hasFrom && hasTo {
		return r, nil
	}

	// "Last" variant
	// TODO is it expected?
	// TODO do not adjust at this time, it should be done by CalculateStepRange
	if hasFrom && !hasTo {
		now := time.Now().Unix()
		r.from = now - r.from
		r.to = now
		return r, nil
	}

	// something wrong
	return r, fmt.Errorf("bad arguments")
}

func parseTimestamp(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseDuration(s string) (int64, error) {
	dur, err := time.ParseDuration(s)
	if err != nil {
		return parseTimestamp(s)
	}
	return int64(dur.Seconds()), nil
}

func decodeMuteDowntimeTypes(in string) []string {
	res := []string{}
	muteTypes := strings.Split(in, "!")
	for _, muteType := range muteTypes {
		switch muteType {
		case "Mnt":
			res = append(res, "Maintenance")
		case "Acd":
			res = append(res, "Accident")
		case "InfMnt":
			res = append(res, "InfrastructureMaintenance")
		case "InfAcd":
			res = append(res, "InfrastructureAccident")
		}
	}
	return res
}
