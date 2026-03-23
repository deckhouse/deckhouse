// Copyright 2025 Flant JSC
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

package main

import (
	"fmt"
	"strings"
	"unicode"
)

func pathSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune('-')
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "object"
	}
	return out
}

// genSubdirForBase picks output folder under test_samples/ from base.document.kind.
func genSubdirForBase(bases map[string]matrixBase, baseName string) string {
	b, ok := bases[baseName]
	if !ok {
		return "other"
	}
	k, _ := b.Document["kind"].(string)
	switch strings.TrimSpace(k) {
	case "Pod":
		return "pods"
	case "SecurityPolicyException":
		return "spe"
	default:
		return "other"
	}
}

// genSeqCounters holds separate ordinal counters per generated output folder (pods / spe / other).
type genSeqCounters struct {
	Pod   int
	Spe   int
	Other int
}

func (g *genSeqCounters) next(sub string) int {
	if g == nil {
		panic("genSeqCounters nil")
	}
	switch sub {
	case "pods":
		g.Pod++
		return g.Pod
	case "spe":
		g.Spe++
		return g.Spe
	default:
		g.Other++
		return g.Other
	}
}

func allocGenPath(seq *genSeqCounters, slugHint string, bases map[string]matrixBase, baseName string) string {
	sub := genSubdirForBase(bases, baseName)
	n := seq.next(sub)
	return fmt.Sprintf("test_samples/%s/%03d-%s.yaml", sub, n, pathSlug(slugHint))
}
