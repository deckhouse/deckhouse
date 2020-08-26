package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// DecodeFromToStep decodes 3 arguments
func DecodeFromToStep(fromArg, toArg, stepArg []string) (from int64, to int64, step int64, err error) {
	now := time.Now().Unix()
	var hasFrom bool
	var hasTo bool
	if len(fromArg) > 0 && fromArg[0] != "" {
		hasFrom = true
		from, err = ParseSecondsOrDuration(fromArg[0])
		if err != nil {
			log.Errorf("parse from='%s' as time duration: %v", fromArg[0], err)
			return 0, 0, 0, err
		}
	}
	if len(toArg) > 0 && toArg[0] != "" {
		hasTo = true
		to, err = ParseSecondsOrDuration(toArg[0])
		if err != nil {
			log.Errorf("parse to='%s' as time duration: %v", toArg[0], err)
			return 0, 0, 0, err
		}
	}
	if len(stepArg) > 0 && stepArg[0] != "" {
		step, err = ParseSecondsOrDuration(stepArg[0])
		if err != nil {
			log.Errorf("parse step='%s' as time duration: %v", stepArg[0], err)
			return 0, 0, 0, err
		}
	} else {
		step = 30
	}

	// "Last" variant
	// TODO is it expected?
	// TODO do not adjust at this time, it should be done by CalculateStepRagnge
	if hasFrom && !hasTo {
		return now - from, now, step, nil
		//return entity.AdjustFrom(now - from), entity.AdjustTo(now), entity.AdjustStep(step), nil
	}
	// "from-to" variant
	if hasFrom && hasTo {
		return from, to, step, nil
		//return entity.AdjustFrom(from), entity.AdjustTo(to), entity.AdjustStep(step), nil
	}
	// something wrong
	return 0, 0, 0, fmt.Errorf("bad arguments")
}

var digitsRe = regexp.MustCompile(`^\d+$`)

func ParseSecondsOrDuration(in string) (int64, error) {
	if digitsRe.MatchString(in) {
		in = in + "s"
	}
	dur, err := time.ParseDuration(in)
	if err != nil {
		return 0, err
	}
	return int64(dur.Seconds()), nil
}

func DecodeMuteDowntimeTypes(in []string) []string {
	defaults := []string{
		"Maintenance",
		"InfraMaintenance",
		"InfraAccident",
	}
	if len(in) == 0 || in[0] == "" {
		return defaults
	}
	res := []string{}
	muteTypes := strings.Split(in[0], ",")
	for _, muteType := range muteTypes {
		switch muteType {
		case "Mnt":
			res = append(res, "Maintenance")
		case "Acd":
			res = append(res, "Accident")
		case "InfMnt":
			res = append(res, "InfraMaintenance")
		case "InfAcd":
			res = append(res, "InfraAccident")
		}
	}
	if len(res) == 0 {
		return defaults
	}
	return res
}
