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

package regexpset

import "testing"

func Test_Match(t *testing.T) {
	tests := []struct {
		name   string
		items  []string
		assert map[string]bool
	}{
		{
			name: "empty",
			assert: map[string]bool{
				"any": false,
			},
		},
		{
			name:  "single item - pure string",
			items: []string{"zz"},
			assert: map[string]bool{
				"zz": true,
				"xx": false,
			},
		},
		{
			name:  "single item - regexp",
			items: []string{"zz-.*"},
			assert: map[string]bool{
				"zz-one": true,
				"zz-two": true,
				"aazz":   false,
				"zzab":   false,
			},
		},
		{
			name:  "reqexp and string items",
			items: []string{"xx", "q.?a", "aa"},
			assert: map[string]bool{
				"xx":          true,
				"qaa":         true,
				"qba":         true,
				"aaz":         true,
				"qxxa":        true,
				"qzza":        false,
				"abracadabra": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withNew, err := New(tt.items...)
			if err != nil {
				t.Errorf("Cannot create with new %v", err)
			}

			withAdd, _ := New()
			err = withAdd.Add(tt.items...)
			if err != nil {
				t.Errorf("Cannot add %v", err)
			}

			for _, r := range []RegExpSet{withAdd, withNew} {
				for x, shouldBe := range tt.assert {
					if shouldBe && !r.Match(x) {
						t.Errorf("missing expected item %s", x)
					}

					if !shouldBe && r.Match(x) {
						t.Errorf("unexpected item %s", x)
					}
				}
			}
		})
	}
}

func Test_ErrorCreatingAdding(t *testing.T) {
	const incorrectRegexp = "a[-wqd"

	_, err := New(incorrectRegexp)
	if err == nil {
		t.Errorf("should returns error")
	}

	s, _ := New()
	err = s.Add(incorrectRegexp)

	if err == nil {
		t.Errorf("should returns error")
	}
}
