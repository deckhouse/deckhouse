package vrl

import (
	"fmt"
	"strings"
)

// Rule is a representation of a VRL rule.
type Rule string

// String returns formatted VRL rule.
func (r Rule) String(args ...interface{}) string {
	return strings.TrimSpace(
		fmt.Sprintf(string(r), args...),
	)
}
