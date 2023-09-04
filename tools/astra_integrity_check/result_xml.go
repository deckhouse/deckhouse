/*
Copyright 2023 Flant JSC

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
	"encoding/xml"
	"fmt"
	"os"
)

type File struct {
	XMLName xml.Name `xml:"file"`
	Name    string   `xml:"filename,attr"`
	Sum     string   `xml:"gostsumorig,attr"`
}

type FilesList struct {
	Files []File
}

type TestResult struct {
	Passed        int       `xml:"-"`
	Failed        int       `xml:"-"`
	NotFound      int       `xml:"-"`
	OKFiles       FilesList `xml:"intOKFiles"`
	FailedFiles   FilesList `xml:"intFailedFiles"`
	NotFoundFiles FilesList `xml:"notFoundFiles"`
}

func (t *TestResult) String() string {
	return fmt.Sprintf("Passed: %d\nFailed: %d\nNot found: %d\n", t.Passed, t.Failed, t.NotFound)
}

func parseResult() (*TestResult, error) {
	resultBin, err := os.ReadFile(resultXMLPath)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile: %w", err)
	}

	result := &TestResult{}
	if err := xml.Unmarshal(resultBin, result); err != nil {
		return nil, fmt.Errorf("xml.Unmarshal: %w", err)
	}
	result.Passed = len(result.OKFiles.Files)
	result.Failed = len(result.FailedFiles.Files)
	result.NotFound = len(result.NotFoundFiles.Files)

	return result, nil
}
