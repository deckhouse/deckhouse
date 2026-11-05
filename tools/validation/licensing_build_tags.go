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

package main

import (
	"errors"
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A Go file named *_ce.go, *_ee.go or *_fe.go, or *_test.go of the same, says by
// its name which editions build it: *_ce.go CE alone, *_ee.go the paid editions,
// *_fe.go FE alone. copy.bara.sky leaves *_ee.go and *_fe.go out of the public
// mirror and mirrors *_ce.go with its build tag stripped, while the build obeys
// the build tag. So the name and the build tag must agree both ways: a file
// named for an edition carries its build tag, and a file whose build tag selects
// editions by the licensing tags is named for that edition (editions.yaml
// goBuildTags).
var editionFileRe = regexp.MustCompile(`_(ce|ee|fe)(_test)?\.go$`)

// editionTagRe finds the tags the editions differ by in a //go:build line, a
// line that does not parse included.
var editionTagRe = regexp.MustCompile(`\b(nolicensing|licensing_optin)\b`)

// editions is the order the editions are matched in.
var editions = []string{"ce", "ee", "fe"}

// editionBuildTags lists the build constraints each suffix accepts. An *_ee.go
// file is built in every edition but CE, or also kept out of FE where FE has an
// *_fe.go of its own (internal/app/licensing_ee.go).
var editionBuildTags = map[string][]string{
	"ce": {"//go:build nolicensing"},
	"ee": {"//go:build !nolicensing", "//go:build !nolicensing && !licensing_optin"},
	"fe": {"//go:build licensing_optin && !nolicensing"},
}

func RunLicensingBuildTagsValidation(info *DiffInfo) (exitCode int) {
	fmt.Printf("Run 'licensing build tags' validation ...\n")

	msgs := NewMessages()
	for _, fileInfo := range info.Files {
		if fileInfo.IsDeleted() {
			continue
		}
		fileName := fileInfo.NewFileName
		if fileName == "" {
			// A pure rename carries no +++ line, and renaming a file into or
			// out of an edition is exactly what has to be caught.
			fileName = fileInfo.HeaderFileName
		}
		if !strings.HasSuffix(fileName, ".go") {
			continue
		}

		src, err := os.ReadFile(fileName)
		if err != nil {
			if editionOf(fileName) != "" {
				msgs.Add(NewSkip(fileName, err.Error()))
			}
			continue
		}
		if msg, checked := checkLicensingBuildTag(fileName, src); checked {
			msgs.Add(msg)
		}
	}
	msgs.PrintReport()

	if msgs.CountErrors() > 0 {
		exitCode = 1
	}
	return exitCode
}

// editionOf returns the edition the file name claims, or "" for none.
func editionOf(fileName string) string {
	if m := editionFileRe.FindStringSubmatch(fileName); m != nil {
		return m[1]
	}
	return ""
}

// checkLicensingBuildTag checks that the name and the build tag of a Go file
// agree. The second result is false for a file that neither claims an edition
// by its name nor names the licensing tags in its //go:build line.
func checkLicensingBuildTag(fileName string, src []byte) (Message, bool) {
	named := editionOf(fileName)
	lines, err := goBuildLines(fileName, src)
	if named == "" && (err != nil || !editionTagRe.MatchString(strings.Join(lines, "\n"))) {
		return Message{}, false
	}
	var got constraint.Expr
	if err == nil {
		got, err = parseGoBuild(lines)
	}

	if named == "" {
		if err != nil {
			return NewError(fileName, "the //go:build line on the licensing tags is broken", err.Error()), true
		}
		edition := editionOfConstraint(got)
		if edition == "" {
			return NewError(fileName, "the build tag selects no edition: use the build tag of *_ce.go, *_ee.go or *_fe.go",
				"got //go:build "+got.String()), true
		}
		return NewError(fileName, fmt.Sprintf("a file with this build tag must be named *_%s.go: %s",
			edition, filepath.Base(editionFileName(fileName, edition))), "got //go:build "+got.String()), true
	}

	accepted := editionBuildTags[named]
	quoted := make([]string, len(accepted))
	for i, tag := range accepted {
		quoted[i] = strconv.Quote(tag)
	}
	msg := fmt.Sprintf("a *_%s.go file must carry %s", named, strings.Join(quoted, " or "))
	switch {
	case err != nil:
		return NewError(fileName, msg, err.Error()), true
	case got == nil:
		return NewError(fileName, msg, "the file has no //go:build line"), true
	case editionOfConstraint(got) != named:
		return NewError(fileName, msg, "got //go:build "+got.String()), true
	}
	return NewOK(fileName), true
}

// editionFileName names the file for an edition: licensing.go becomes
// licensing_ee.go, licensing_test.go becomes licensing_ee_test.go.
func editionFileName(fileName, edition string) string {
	if stem, ok := strings.CutSuffix(fileName, "_test.go"); ok {
		return stem + "_" + edition + "_test.go"
	}
	return strings.TrimSuffix(fileName, ".go") + "_" + edition + ".go"
}

// goBuildLines returns the //go:build lines of a Go file. As for the go
// command, they are line comments before the package clause: the same text in
// a block comment or below the package clause does not count.
func goBuildLines(fileName string, src []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), fileName, src, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() < file.Package && constraint.IsGoBuild(comment.Text) {
				lines = append(lines, comment.Text)
			}
		}
	}
	return lines, nil
}

// parseGoBuild parses the //go:build line of a file, nil without one. The go
// command refuses a file whose line does not parse, and so does this check.
func parseGoBuild(lines []string) (constraint.Expr, error) {
	switch len(lines) {
	case 0:
		return nil, nil
	case 1:
		x, err := constraint.Parse(lines[0])
		if err != nil {
			return nil, fmt.Errorf("%q does not parse: %w", lines[0], err)
		}
		return x, nil
	default:
		return nil, errors.New("more than one //go:build line")
	}
}

// editionOfConstraint returns the edition whose build tag the constraint is, or
// "" for none.
func editionOfConstraint(x constraint.Expr) string {
	for _, edition := range editions {
		for _, tag := range editionBuildTags[edition] {
			if want, err := constraint.Parse(tag); err == nil && sameEditions(x, want) {
				return edition
			}
		}
	}
	return ""
}

// sameEditions reports whether two build constraints build a file in the same
// editions: they agree on every combination of the two licensing tags. Other
// tags count as set, they do not tell editions apart.
func sameEditions(a, b constraint.Expr) bool {
	for _, nolicensing := range []bool{false, true} {
		for _, optin := range []bool{false, true} {
			tags := func(tag string) bool {
				switch tag {
				case "nolicensing":
					return nolicensing
				case "licensing_optin":
					return optin
				default:
					return true
				}
			}
			if a.Eval(tags) != b.Eval(tags) {
				return false
			}
		}
	}
	return true
}
