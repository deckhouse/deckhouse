/*
Copyright 2021 Flant JSC

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

package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrWhenExactlyOneOperator = errors.New("when: exactly one operator is allowed; quote the value if it must contain operator characters")

type WhenOp string

const (
	WhenEQ        WhenOp = "=="
	WhenNE        WhenOp = "!="
	WhenRe        WhenOp = "=~"
	WhenNRe       WhenOp = "!=~"
	WhenExists    WhenOp = "exists"
	WhenNotExists WhenOp = "notexists"
)

var whenOpsParseOrder = []WhenOp{WhenNRe, WhenRe, WhenNE, WhenEQ}

type WhenExpr struct {
	LeftPath     string
	LeftPathSegs []string
	Op           WhenOp
	Value        string
}

func ParseWhen(s string) (*WhenExpr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("when: empty expression")
	}
	for _, op := range whenOpsParseOrder {
		os := string(op)
		pos := strings.Index(s, os)
		if pos <= 0 {
			continue
		}
		opLen := len(os)
		left := strings.TrimSpace(s[:pos])
		right := s[pos+opLen:]
		val, err := parseRightValue(right)
		if err != nil {
			return nil, err
		}
		segs, err := ParseLabelPath(left)
		if err != nil {
			return nil, fmt.Errorf("when left path: %w", err)
		}
		expr := &WhenExpr{LeftPath: left, LeftPathSegs: segs, Op: op, Value: val}
		if op == WhenEQ || op == WhenNE {
			if _, ok := MatchMustachePath(strings.TrimSpace(val)); ok {
				return nil, fmt.Errorf("when: mustache path templates are not allowed")
			}
		}
		return expr, nil
	}
	if strings.HasPrefix(s, "!.") {
		rest := strings.TrimSpace(s[1:])
		if rest != "" {
			segs, err := ParseLabelPath(rest)
			if err == nil {
				return &WhenExpr{LeftPath: rest, LeftPathSegs: segs, Op: WhenNotExists}, nil
			}
		}
	}
	if segs, err := ParseLabelPath(s); err == nil {
		return &WhenExpr{LeftPath: s, LeftPathSegs: segs, Op: WhenExists}, nil
	}
	return nil, fmt.Errorf("invalid when expression")
}

func parseRightValue(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("when: empty when value")
	}
	if strings.HasPrefix(s, "'") || strings.HasPrefix(s, `"`) {
		end, seg, err := readQuotedSegment(s, s[0])
		if err != nil {
			return "", fmt.Errorf("when: %w", err)
		}
		if end != len(s) {
			return "", ErrWhenExactlyOneOperator
		}
		return seg, nil
	}
	return s, nil
}

// ValidateWhenRegexExpr compiles WhenExpr.Value for =~ / !=~.
// Invalid patterns must be rejected when building config so Vector does not fail at runtime.
func ValidateWhenRegexExpr(we *WhenExpr) error {
	_, err := regexp.Compile(we.Value)
	if err != nil {
		return fmt.Errorf("when: invalid regex: %w", err)
	}
	return nil
}

// VRLRegexFindComparisonOp maps =~ / !=~ to the equality operator used in generated VRL
// for regex match vs non-match on the extracted string.
func VRLRegexFindComparisonOp(op WhenOp) (string, error) {
	switch op {
	case WhenRe:
		return "==", nil
	case WhenNRe:
		return "!=", nil
	default:
		return "", fmt.Errorf("when: unsupported regex op %q", op)
	}
}
