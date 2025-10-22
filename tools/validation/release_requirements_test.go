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

package main

import (
	"fmt"
	"testing"
)

var testRequirementsPackage = "test"

func Test_areNewRequirementsChecked(t *testing.T) {
	_, _, err := checksAndRequirements(map[string]struct{}{}, "testdata/release_requirements/correct/example.go", testRequirementsPackage)
	if err != nil {
		t.Errorf("Should parse correct/example.go file successfully: %s", err)
	}

	_, _, err = checksAndRequirements(map[string]struct{}{}, "testdata/release_requirements/faulty/function-assignment.go", testRequirementsPackage)
	if err == nil {
		t.Errorf("Should fail to parse faulty/function-assignment.go file: %s", err)
	}

	prematureChecks, _, err := checksAndRequirements(map[string]struct{}{"testVer": struct{}{}}, "testdata/release_requirements/faulty/example.go", testRequirementsPackage)
	if err != nil {
		t.Errorf("Should parse faulty/example.go file successfully")
	}

	if len(prematureChecks) == 0 {
		t.Errorf("List of premature checks shouldn't be of 0 length")
	}

	prematureChecks, eligibleChecks, err := checksAndRequirements(map[string]struct{}{"testVer": struct{}{}}, "testdata/release_requirements/faulty/extra-check.go", testRequirementsPackage)
	if err != nil {
		t.Errorf("Should parse faulty/extra-check.go file successfully")
	}

	if len(prematureChecks) == 0 {
		t.Errorf("List of premature checks shouldn't be of 0 length")
	}

	if len(eligibleChecks) == 0 {
		t.Errorf("List of eligible checks shouldn't be of 0 length")
	}
}

func Test_getRequirements(t *testing.T) {
	lines := []string{
		`  "testVer": "1.16" # modules/110-istio/requirements/check.go`,
	}
	expect := map[string]struct{}{"testVer": struct{}{}}

	_, requirements, err := getRequirements(lines, "testdata/release_requirements/release.yaml")
	if err != nil {
		t.Errorf("Should get requirements without an error: %s", err)
	}

	if fmt.Sprint(requirements) != fmt.Sprint(expect) {
		t.Errorf("Expect '%s', got '%s'", expect, requirements)
	}

	lines = []string{
		`  "testVer": "1.16" # modules/110-istio/requirements/check.go`,
		`  "istioVer": "1.9"`,
	}
	expect = map[string]struct{}{"testVer": struct{}{}, "istioVer": struct{}{}}

	_, requirements, err = getRequirements(lines, "testdata/release_requirements/release.yaml")
	if err != nil {
		t.Errorf("Should get requirements without an error: %s", err)
	}

	if fmt.Sprint(requirements) != fmt.Sprint(expect) {
		t.Errorf("Expect '%s', got '%s'", expect, requirements)
	}

	lines = []string{}
	expect = map[string]struct{}{}

	_, requirements, err = getRequirements(lines, "testdata/release_requirements/release.yaml")
	if err != nil {
		t.Errorf("Should get requirements without an error: %s", err)
	}

	if fmt.Sprint(requirements) != fmt.Sprint(expect) {
		t.Errorf("Expect '%s', got '%s'", expect, requirements)
	}
}
