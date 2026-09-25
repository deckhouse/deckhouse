// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"strings"
)

// configurationFailure renders a configuration error the way a preflight failure reads: what was
// looked at, what came back, what would have passed, and what to change.
//
// The shape is copied rather than imported. These checks used to be preflight checks and were
// moved here so they also run for `dhctl config` and for converge, and so no skip flag turns them
// off — but pkg/preflight imports this package, so the type cannot travel in the other direction.
// What matters to the reader is that one paragraph of prose became four labelled lines, the same
// four they see for every other check.
func configurationFailure(checked, observed, expected, fix string) error {
	var b strings.Builder

	writeConfigField(&b, "checked", checked)
	writeConfigField(&b, "observed", observed)
	writeConfigField(&b, "expected", expected)
	writeConfigField(&b, "fix", fix)

	return fmt.Errorf("%s", strings.TrimRight(b.String(), "\n"))
}

// writeConfigField indents the continuation lines of a multi-line value under its label, so a
// value that spans lines still reads as one field.
func writeConfigField(b *strings.Builder, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	indent := strings.Repeat(" ", len(name)+2)
	fmt.Fprintf(b, "%s: %s\n", name, strings.ReplaceAll(value, "\n", "\n"+indent))
}
