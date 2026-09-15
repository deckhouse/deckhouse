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

package preflightnew

type Phase string

const (
	PhasePreInfra  Phase = "pre-infra"
	PhasePostInfra Phase = "post-infra"
)

// phaseFormats is the title of the process box. It names what the phase looks at, and carries
// the --skip-phase spelling in parentheses so the box and the flag that skips it can be matched
// without consulting the documentation. The previous titles — "Settings preflights" and "Infra
// preflights" — matched neither the flag nor, on a static cluster, the truth: "Infra preflights"
// ran with no infrastructure between it and the phase before.
var phaseFormats = map[Phase]string{
	PhasePreInfra:  "Preflight checks: configuration (PreInfraPreflights)",
	PhasePostInfra: "Preflight checks: nodes (PostInfraPreflights)",
}

func (p Phase) String() string {
	if f, ok := phaseFormats[p]; ok {
		return f
	}

	return string(p)
}

func (p Phase) FormatString() string {
	return p.String()
}
