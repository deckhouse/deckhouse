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

package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type DiffInfo struct {
	Files []*DiffFileInfo
}

func NewDiffInfo() *DiffInfo {
	return &DiffInfo{
		Files: make([]*DiffFileInfo, 0),
	}
}

func (d *DiffInfo) Dump() string {
	res := ""
	for _, info := range d.Files {
		res += fmt.Sprintf("%s -> %s, lines: %d\n", info.OldFileName, info.NewFileName, len(info.Lines))
	}
	res += fmt.Sprintf("files: %d\n", len(d.Files))
	return res
}

type DiffFileInfo struct {
	NewFileName string
	OldFileName string
	Lines       []string
}

func (d *DiffFileInfo) IsAdded() bool {
	return d.OldFileName == "/dev/null"
}

func (d *DiffFileInfo) IsDeleted() bool {
	return d.NewFileName == "/dev/null"
}

func (d *DiffFileInfo) IsModified() bool {
	return d.OldFileName != "/dev/null" && d.NewFileName != "/dev/null" && d.HasContent()
}

func (d *DiffFileInfo) HasContent() bool {
	return len(d.Lines) > 0
}

func (d *DiffFileInfo) NewLines() []string {
	res := make([]string, 0)
	for _, l := range d.Lines {
		if strings.HasPrefix(l, "+") {
			res = append(res, strings.TrimPrefix(l, "+"))
		}
	}
	return res
}

func NewDiffFileInfo() *DiffFileInfo {
	return &DiffFileInfo{
		Lines: make([]string, 0),
	}
}

var diffStartRe = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)
var oldFileNameRe = regexp.MustCompile(`^--- (/dev/null|a/(.*))$`)
var newFileNameRe = regexp.MustCompile(`^\+\+\+ (/dev/null|b/(.*))$`)
var endMetadataRe = regexp.MustCompile(`^@@[\-+ \d,]+@@(.*)$`)

func ParseDiffOutput(r io.Reader) (*DiffInfo, error) {
	res := NewDiffInfo()
	tmp := NewDiffFileInfo()
	firstLine := true
	scanner := bufio.NewScanner(r)
	metadataBlock := false
	for scanner.Scan() {
		text := scanner.Text()

		if diffStartRe.MatchString(text) {
			if firstLine {
				firstLine = false
			} else {
				// Append diffFileInfo when all lines are gathered and new diffFIleInfo is detected.
				res.Files = append(res.Files, tmp)
				tmp = NewDiffFileInfo()
			}
			metadataBlock = true
			continue
		}

		matches := newFileNameRe.FindStringSubmatch(text)
		if len(matches) > 1 {
			if matches[1] == "/dev/null" {
				tmp.NewFileName = matches[1]
			} else {
				tmp.NewFileName = matches[2]
			}
			continue
		}

		matches = oldFileNameRe.FindStringSubmatch(text)
		if len(matches) > 1 {
			if matches[1] == "/dev/null" {
				tmp.OldFileName = matches[1]
			} else {
				tmp.OldFileName = matches[2]
			}
			continue
		}

		if metadataBlock {
			matches = endMetadataRe.FindStringSubmatch(text)
			if len(matches) > 1 {
				tmp.Lines = append(tmp.Lines, matches[1])
				metadataBlock = false
				continue
			}
		}

		if !metadataBlock {
			tmp.Lines = append(tmp.Lines, text)
		}
	}
	// Push last diff info.
	if tmp != nil {
		res.Files = append(res.Files, tmp)
	}

	return res, nil
}
