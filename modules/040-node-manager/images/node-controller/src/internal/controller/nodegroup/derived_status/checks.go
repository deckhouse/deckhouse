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

package derived_status

import (
	"fmt"
	"sort"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

type CloudCheckInput struct {
	NodeType  v1.NodeType
	KindInUse string

	ClassRefKind    string
	ClassRefName    string
	KnownClassNames []string

	MinPerZone  int32
	MaxPerZone  int32
	CapacityErr error

	SpecZones    []string
	DefaultZones []string
}

type CloudCheckResult struct {
	Processed bool
	Error     string
}

func RunCloudChecks(in CloudCheckInput) CloudCheckResult {
	if in.NodeType != v1.NodeTypeCloudEphemeral || in.KindInUse == "" {
		return CloudCheckResult{Processed: false}
	}

	// check #1 — classReference.kind must be the kind allowed in the cluster.
	if in.ClassRefKind != in.KindInUse {
		return CloudCheckResult{Error: fmt.Sprintf(
			"Invalid classReference.kind '%s'. Expected '%s'. Please update the NodeGroup to use the correct instance class kind.",
			in.ClassRefKind, in.KindInUse)}
	}

	// check #2 — classReference must point to an existing instance class.
	if !containsString(in.KnownClassNames, in.ClassRefName) {
		return CloudCheckResult{Error: fmt.Sprintf(
			"Instance class '%s' of type '%s' not found. Please create the required instance class or update the NodeGroup to reference an existing one.",
			in.ClassRefName, in.ClassRefKind)}
	}

	// check #3 — scale-from-zero requires a resolvable node capacity.
	if in.MinPerZone == 0 && in.MaxPerZone > 0 && in.CapacityErr != nil {
		return CloudCheckResult{Error: fmt.Sprintf(
			"Capacity calculation failed for instance class '%s'. The instance type is not found in built-in types and no capacity is set. ScaleFromZero will not work. Please set capacity in the %s '%s' or use a supported instance type.",
			in.ClassRefKind, in.ClassRefKind, in.ClassRefName)}
	}

	// check #4 — spec zones must all be known default zones.
	if len(in.DefaultZones) > 0 {
		known := make(map[string]struct{}, len(in.DefaultZones))
		for _, z := range in.DefaultZones {
			known[z] = struct{}{}
		}
		unknownZones := make([]string, 0)
		for _, zone := range in.SpecZones {
			if _, ok := known[zone]; !ok {
				unknownZones = append(unknownZones, zone)
			}
		}
		if len(unknownZones) > 0 {
			return CloudCheckResult{Error: fmt.Sprintf(
				"Invalid zones specified: %v. Available zones: %v. Please update the NodeGroup to use valid zones.",
				unknownZones, sortedCopy(in.DefaultZones))}
		}
	}

	return CloudCheckResult{Processed: true}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// sortedCopy mirrors set.Slice() ordering in the check #4 error message.
func sortedCopy(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
