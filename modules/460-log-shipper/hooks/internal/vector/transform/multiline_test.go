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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"

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
				NotRegex: pointer.String("^notRegex"),
			},
			want: pointer.String("matched, err = match(.message, r'^notRegex');\nif err != null {\n    true;\n} else {\n    !matched;\n}"),
		},
		{
			name: "regex",
			parserRegex: &v1alpha1.ParserRegex{
				Regex: pointer.String("^regex"),
			},
			want: pointer.String("matched, err = match(.message, r'^regex');\nif err != null {\n    false;\n} else {\n    matched;\n}"),
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
			name: "regex::and::notRegex::not::nil",
			parserRegex: &v1alpha1.ParserRegex{
				Regex:    pointer.String("^regex"),
				NotRegex: pointer.String("^notRegex"),
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
			fmt.Printf("%q\n", *got)
			assert.Equal(t, tt.want, got)
		})
	}
}
