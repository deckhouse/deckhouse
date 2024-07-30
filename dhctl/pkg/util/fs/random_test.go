// Copyright 2023 Flant JSC
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

package fs

import (
	"testing"
)

const selectionSize = 10000

func TestRandomTmpFileName(t *testing.T) {
	set := make(map[string]bool, selectionSize) // Use a map as a set
	for i := 0; i < selectionSize; i++ {
		set[RandomTmpFileName()] = true
	}
	difference := selectionSize - len(set)
	if difference > 0 {
		t.Errorf("random filenames have %d repetitions out of %d elements", difference, selectionSize)
	}
}

// technically, this one has already being tested as part of TestRandomTmpFileName
func TestRandomNumberSuffix(t *testing.T) {
	set := make(map[string]bool, selectionSize) // Use a map as a set
	for i := 0; i < selectionSize; i++ {
		set[RandomNumberSuffix("dhctl-tst-touch")] = true
	}
	difference := selectionSize - len(set)
	if difference > 0 {
		t.Errorf("random filenames have %d repetitions out of %d elements", difference, selectionSize)
	}
}
