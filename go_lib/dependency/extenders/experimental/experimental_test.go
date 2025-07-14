/*
Copyright 2025 Flant JSC

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
package experimental

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestExtender_IsTerminator(t *testing.T) {
	e := NewExtender(false, log.NewNop())
	if !e.IsTerminator() {
		t.Fatalf("expected IsTerminator() to return true")
	}
}

func TestExtender_Filter(t *testing.T) {
	type tc struct {
		name             string
		allow            bool
		markExperimental bool
		wantDecision     *bool
	}

	tests := []tc{
		{
			name:             "experimental forbidden when flag is false",
			allow:            false,
			markExperimental: true,
			wantDecision:     ptr.To(false),
		},
		{
			name:             "non-experimental allowed when flag is false",
			allow:            false,
			markExperimental: false,
			wantDecision:     nil,
		},
		{
			name:             "experimental allowed when flag is true",
			allow:            true,
			markExperimental: true,
			wantDecision:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNop()
			e := NewExtender(tt.allow, logger)

			const moduleName = "test-module"
			if tt.markExperimental {
				e.AddConstraint(moduleName)
			}

			got, _ := e.Filter(moduleName, nil)

			switch {
			case got == nil && tt.wantDecision != nil:
				t.Fatalf("got nil decision, want %v", *tt.wantDecision)
			case got != nil && tt.wantDecision == nil:
				t.Fatalf("got %#v, want nil decision", *got)
			case got != nil && *got != *tt.wantDecision:
				t.Fatalf("got decision %v, want %v", *got, *tt.wantDecision)
			}
		})
	}
}
