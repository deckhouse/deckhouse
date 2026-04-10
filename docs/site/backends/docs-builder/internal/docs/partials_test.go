// Copyright 2026 Flant JSC
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

package docs

import "testing"

func TestValidatePartialPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "root partial", path: "/partials/setup-guide.md"},
		{name: "nested partial", path: "/partials/setup/basic-setup-v2.ru.md"},
		{name: "static asset", path: "/partials/static/images/diagram.png"},
		{name: "nested static directory", path: "/partials/setup/static/diagram.md", wantErr: true},
		{name: "uppercase file", path: "/partials/Setup.md", wantErr: true},
		{name: "underscore", path: "/partials/setup_guide.md", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePartialPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePartialPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
