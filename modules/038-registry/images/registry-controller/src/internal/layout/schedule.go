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

package layout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DefaultGarbageCollectionSchedule runs shortly after 03:00, every day.
//
// A night hour because collecting puts a replica read-only for the duration: it keeps serving
// every image it holds, but it cannot store the result of a cache miss and cannot accept a
// `d8 mirror push`. Neither is an outage, and both are better placed where the cluster is
// quiet.
const DefaultGarbageCollectionSchedule = "17 3 * * *"

// MaintenanceWindow is a period an operator has already declared safe for disruption.
type MaintenanceWindow struct {
	// From and To are wall-clock times, "HH:MM".
	From string
	To   string

	// Days are the weekdays the window applies to, as three-letter names. Empty means
	// every day.
	Days []string
}

// weekday maps the names a NodeGroup window uses onto cron's numbering.
var weekday = map[string]int{
	"Sun": 0, "Mon": 1, "Tue": 2, "Wed": 3, "Thu": 4, "Fri": 5, "Sat": 6,
}

// GarbageCollectionSchedule decides when a replica collects its store.
//
// Three sources, in order of how much they know about this particular cluster:
//
//  1. what an operator configured, which is the only source that knows their intent;
//  2. the maintenance window of the master node group, which is when they have already said
//     disruption is acceptable — and collecting is a small disruption of exactly that kind;
//  3. a night hour, because something has to be chosen and the middle of the night is the
//     least bad guess.
//
// The window is used at its start rather than somewhere inside it: a collection that begins
// as the window opens has the whole window to finish in, and one scheduled in the middle may
// still be running when the window closes.
func GarbageCollectionSchedule(configured string, windows []MaintenanceWindow) string {
	if trimmed := strings.TrimSpace(configured); trimmed != "" {
		return trimmed
	}

	for _, window := range windows {
		schedule, err := window.cron()
		if err != nil {
			// A window that cannot be read is not a reason to refuse to collect. It is a
			// reason to fall back to the night default, which is what an unconfigured
			// cluster gets anyway.
			continue
		}
		return schedule
	}

	return DefaultGarbageCollectionSchedule
}

// cron renders the window's opening as a cron expression.
func (w MaintenanceWindow) cron() (string, error) {
	hour, minute, err := parseClock(w.From)
	if err != nil {
		return "", err
	}

	days, err := cronDays(w.Days)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d %d * * %s", minute, hour, days), nil
}

// parseClock reads an "HH:MM" wall-clock time.
func parseClock(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%q is not a time of day", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("%q has no valid hour", value)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("%q has no valid minute", value)
	}
	return hour, minute, nil
}

// cronDays renders the weekday list, sorted so that the same window always produces the same
// expression — otherwise the schedule would appear to change on every reconciliation and the
// storage spec would never settle.
func cronDays(days []string) (string, error) {
	if len(days) == 0 {
		return "*", nil
	}

	numbers := make([]int, 0, len(days))
	seen := map[int]struct{}{}
	for _, day := range days {
		number, ok := weekday[strings.TrimSpace(day)]
		if !ok {
			return "", fmt.Errorf("%q is not a weekday", day)
		}
		if _, duplicate := seen[number]; duplicate {
			continue
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}

	sort.Ints(numbers)

	rendered := make([]string, 0, len(numbers))
	for _, number := range numbers {
		rendered = append(rendered, strconv.Itoa(number))
	}
	return strings.Join(rendered, ","), nil
}
