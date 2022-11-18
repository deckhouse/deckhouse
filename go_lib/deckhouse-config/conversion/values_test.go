/*
Copyright 2022 Flant JSC

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

	. "github.com/onsi/gomega"
)

func TestJSONValuesNew(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			"nil",
			nil,
			`null`,
		},
		{
			"empty",
			map[string]interface{}{},
			`{}`,
		},
		{
			"one field",
			map[string]interface{}{"auth": 1},
			`{"auth":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			vals, err := SettingsFromMap(tt.input)
			g.Expect(err).ShouldNot(HaveOccurred(), "should adopt input")
			g.Expect(vals.String()).Should(Equal(tt.expect))

			err = vals.Set("newField", "newValue")
			g.Expect(err).ShouldNot(HaveOccurred(), "should set newField")
			val := vals.Get("newField")
			g.Expect(val.String()).To(Equal("newValue"))
		})
	}
}

func TestJSONValuesDeleteEmpty(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		path   string
		expect string
	}{
		{
			"empty object",
			`{"auth":{}}`,
			"auth",
			`{}`,
		},
		{
			"empty array",
			`{"auth":[]}`,
			"auth",
			`{}`,
		},
		{
			"null",
			`{"auth":null}`,
			"auth",
			`{"auth":null}`,
		},
		{
			"nonexistent",
			`{"param":null}`,
			"auth",
			`{"param":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			vals := SettingsFromBytes([]byte(tt.input))
			err := vals.DeleteIfEmptyParent(tt.path)
			g.Expect(err).ShouldNot(HaveOccurred(), "should delete path")
			g.Expect(vals.String()).Should(Equal(tt.expect))
		})
	}
}

func TestJSONValuesDeleteAndClean(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		path   string
		expect string
	}{
		{
			"empty object",
			`{"auth":{}}`,
			"auth",
			`{}`,
		},
		{
			"empty array",
			`{"auth":[]}`,
			"auth",
			`{}`,
		},
		{
			"null",
			`{"auth":null}`,
			"auth",
			`{}`,
		},
		{
			"nonexistent",
			`{"param":null}`,
			"auth",
			`{"param":null}`,
		},
		{
			"parent becomes an empty object",
			`{"auth":{"password":"p4ssw0rd"}}`,
			"auth.password",
			`{}`,
		},
		{
			"parent becomes an empty array",
			`{"auth":["password"]}`,
			"auth.0",
			`{}`,
		},
		{
			"parents become empty",
			`{"auth":{"passwords":["p4ssw0rd"]}}`,
			"auth.passwords.0",
			`{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			vals := SettingsFromBytes([]byte(tt.input))
			err := vals.DeleteAndClean(tt.path)
			g.Expect(err).ShouldNot(HaveOccurred(), "should delete path")
			g.Expect(vals.String()).Should(Equal(tt.expect))
		})
	}
}
