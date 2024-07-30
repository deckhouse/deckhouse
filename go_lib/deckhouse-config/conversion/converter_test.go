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

package conversion

import (
	"testing"
)

func TestConvertToLatest(t *testing.T) {
	t.Run("should convert from 1 to latest version", func(t *testing.T) {
		err := TestConvert(
			`
simpleField: simpleValue
objectField:
  arrayFieldInObjectValue:
    - value1
    - value2
    - value3
  mapFieldInObjectValue:
    key1: val1
    key2: val2
  simpleFieldInObjectValue: 2
arrayField:
  - val1
  - val2
  - val3
mapField:
  simpleFieldInObjectValue: 2
  arrayFieldInMapValue:
    - v1
    - v2
    - v3
  mapFieldInObjectValue:
    k1: 1
    k2: 2
objectFirst:
  check: true
objectForDelete:
  key: val
`,
			`
simpleField: value
objectField:
  arrayFieldInObjectValue:
    - value1
    - value2
    - value3
  mapFieldInObjectValue:
    key1: val2
  simpleFieldInObjectValue: 2
mapField:
  arrayFieldInMapValue:
    - v1
    - v2
    - v3
    - v4
  mapFieldInObjectValue:
    k1: 1
    k2: 2
    k3: 3
newField: value
objectFirst:
  objectSecond:
    objectThird:
      check: true
arrayField:
  - v1
  - v2
  - v3
`,
			"testdata",
			1,
			0)
		if err != nil {
			t.Error(err)
		}
	})
}
