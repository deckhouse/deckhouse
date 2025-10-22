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

package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func Test_processMultilineRegex(t *testing.T) {
	tests := []struct {
		name        string
		parserRegex *v1alpha1.ParserRegex
		want        *string
		wantErr     string
	}{
		{
			name: "notRegex",
			parserRegex: &v1alpha1.ParserRegex{
				NotRegex: ptr.To("^notRegex"),
			},
			want: ptr.To("matched, err = match(.message, r'^notRegex');\nif err != null {\n    true;\n} else {\n    !matched;\n}"),
		},
		{
			name: "regex",
			parserRegex: &v1alpha1.ParserRegex{
				Regex: ptr.To("^regex"),
			},
			want: ptr.To("matched, err = match(.message, r'^regex');\nif err != null {\n    false;\n} else {\n    matched;\n}"),
		},
		{
			name:        "nil",
			parserRegex: nil,
			wantErr:     "no regex provided",
		},
		{
			name:        "nil::regex::and::notRegex",
			parserRegex: &v1alpha1.ParserRegex{},
			wantErr:     "regex or notRegex should be provided",
		},
		{
			name: "not::nil::regex::and::notRegex",
			parserRegex: &v1alpha1.ParserRegex{
				Regex:    ptr.To("^regex"),
				NotRegex: ptr.To("^notRegex"),
			},
			wantErr: "must be set one of regex or notRegex",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processMultilineRegex(tt.parserRegex)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_processCustomMultiLIneTransform(t *testing.T) {
	tests := []struct {
		name                  string
		multilineCustomConfig v1alpha1.MultilineParserCustom
		expected              map[string]interface{}
		wantErr               string
	}{
		{
			name:                  "nil::startsWhen::and::endsWhen",
			multilineCustomConfig: v1alpha1.MultilineParserCustom{},
			wantErr:               "no values provided in multilineParser.custom",
		},
		{
			name: "not::nil::startsWhen::and::endsWhen",
			multilineCustomConfig: v1alpha1.MultilineParserCustom{
				StartsWhen: &v1alpha1.ParserRegex{
					Regex: ptr.To("startsWhen"),
				},
				EndsWhen: &v1alpha1.ParserRegex{
					NotRegex: ptr.To("endsWhen"),
				},
			},
			wantErr: "provide one of endsWhen or startsWhen in multilineParser.custom",
		},
		{
			name: "not::nil::startsWhen",
			multilineCustomConfig: v1alpha1.MultilineParserCustom{
				StartsWhen: &v1alpha1.ParserRegex{
					Regex: ptr.To("startsWhen"),
				},
			},
			expected: map[string]interface{}{
				"starts_when": "matched, err = match(.message, r'startsWhen');\nif err != null {\n    false;\n} else {\n    matched;\n}",
			},
		},
		{
			name: "not::nil::endsWhen",
			multilineCustomConfig: v1alpha1.MultilineParserCustom{
				EndsWhen: &v1alpha1.ParserRegex{
					NotRegex: ptr.To("endsWhen"),
				},
			},
			expected: map[string]interface{}{
				"ends_when": "matched, err = match(.message, r'endsWhen');\nif err != null {\n    true;\n} else {\n    !matched;\n}",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]interface{})
			err := processCustomMultiLIneTransform(tt.multilineCustomConfig, result)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
