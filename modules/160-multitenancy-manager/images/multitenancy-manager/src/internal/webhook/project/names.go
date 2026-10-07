/*
Copyright 2024 Flant JSC

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

package project

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"controller/apis/deckhouse.io/v1alpha3"
)

// nameWarnings returns the admission warnings for a project name that reads like an additional
// namespace of another project, or that makes existing projects read like its own additional
// namespaces: "<p>-<suffix>" beside "<p>". Such projects are separate, and nothing about them
// changes, but the name suggests otherwise to whoever reads it. Virtual projects have no
// additional namespaces to be confused with, and projects being deleted are left out.
func nameWarnings(name string, existing []v1alpha3.Project) []string {
	var prefixes, extending []string
	for i := range existing {
		other := &existing[i]
		if other.Name == name || other.IsVirtual() || !other.DeletionTimestamp.IsZero() {
			continue
		}
		switch {
		case strings.HasPrefix(name, other.Name+"-"):
			prefixes = append(prefixes, other.Name)
		case strings.HasPrefix(other.Name, name+"-"):
			extending = append(extending, strconv.Quote(other.Name))
		}
	}
	slices.Sort(prefixes)
	slices.Sort(extending)

	warnings := make([]string, 0, len(prefixes)+1)
	for _, prefix := range prefixes {
		warnings = append(warnings, fmt.Sprintf(
			"Project %q is not an additional namespace of project %q. It is created as a separate project with a namespace of its own. "+
				"To add a namespace to project %q, create a ProjectNamespace in the %q namespace instead.",
			name, prefix, prefix, prefix))
	}
	if len(extending) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"The existing projects named like additional namespaces of project %q are separate projects: %s.",
			name, strings.Join(extending, ", ")))
	}
	return warnings
}
