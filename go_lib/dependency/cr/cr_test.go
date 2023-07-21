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

package cr

import (
	"testing"
)

func TestParse(t *testing.T) {
	testURL := "registry.deckhouse.io/deckhouse/fe"
	u, err := parse(testURL)
	if err != nil {
		t.Errorf("got error: %s", err)
	}
	if u.String() != "//"+testURL {
		t.Errorf("got: %s, wanted: %s", u, testURL)
	}

	testURL = "registry.deckhouse.io:5123/deckhouse/fe"
	u, err = parse(testURL)
	if err != nil {
		t.Errorf("got error: %s", err)
	}
	if u.String() != "//"+testURL {
		t.Errorf("got: %s, wanted: %s", u, testURL)
	}
}

func TestAddTrailingDot(t *testing.T) {
	testURL := "registry.deckhouse.io/deckhouse/fe"
	u, err := addTrailingDot(testURL)
	if err != nil {
		t.Errorf("got error: %s", err)
	}
	if u != "registry.deckhouse.io./deckhouse/fe" {
		t.Errorf("got: %s, wanted: %s", u, testURL)
	}

	testURL = "registry.deckhouse.io:5000/deckhouse/fe"
	u, err = addTrailingDot(testURL)
	if err != nil {
		t.Errorf("got error: %s", err)
	}
	if u != "registry.deckhouse.io.:5000/deckhouse/fe" {
		t.Errorf("got: %s, wanted: %s", u, testURL)
	}
}
