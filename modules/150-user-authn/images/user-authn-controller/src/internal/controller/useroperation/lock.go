/*
Copyright 2026 Flant JSC

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

package useroperation

import (
	"regexp"
	"strconv"
	"time"
)

// lockDaysSegment matches one "<number>d" segment in a duration string.
var lockDaysSegment = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)d`)

// expandLockDuration rewrites every "<num>d" segment in a Go-style duration
// string into the equivalent "<num*24>h" so time.ParseDuration — which has
// no "d" unit — can consume the result.
func expandLockDuration(s string) string {
	return lockDaysSegment.ReplaceAllStringFunc(s, func(seg string) string {
		days, err := strconv.ParseFloat(seg[:len(seg)-1], 64)
		if err != nil {
			return seg
		}
		return strconv.FormatFloat(days*24, 'f', -1, 64) + "h"
	})
}

// resolveLockUntil maps lock.for to an absolute expiry.
func resolveLockUntil(forValue string, now time.Time) (time.Time, error) {
	if forValue == lockForever {
		return foreverTime, nil
	}
	d, err := time.ParseDuration(expandLockDuration(forValue))
	if err != nil {
		return time.Time{}, failedf("invalid lock.for %q: %s", forValue, err.Error())
	}
	if d <= 0 {
		return time.Time{}, failedf("lock.for %q must be a positive duration", forValue)
	}
	return now.Add(d), nil
}
