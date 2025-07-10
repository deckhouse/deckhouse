// experimental_test.go
package experimental

import (
	"errors"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"
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
		wantErr          bool
	}

	tests := []tc{
		{
			name:             "experimental forbidden when flag is false",
			allow:            false,
			markExperimental: true,
			wantDecision:     ptr.To(false),
			wantErr:          true,
		},
		{
			name:             "non-experimental allowed when flag is false",
			allow:            false,
			markExperimental: false,
			wantDecision:     nil,
			wantErr:          false,
		},
		{
			name:             "experimental allowed when flag is true",
			allow:            true,
			markExperimental: true,
			wantDecision:     ptr.To(true),
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNop()
			e := NewExtender(tt.allow, logger)

			const moduleName = "test-module"
			if tt.markExperimental {
				e.AddConstraint(moduleName)
			}

			got, err := e.Filter(moduleName, nil)

			switch {
			case got == nil && tt.wantDecision != nil:
				t.Fatalf("got nil decision, want %v", *tt.wantDecision)
			case got != nil && tt.wantDecision == nil:
				t.Fatalf("got %#v, want nil decision", *got)
			case got != nil && *got != *tt.wantDecision:
				t.Fatalf("got decision %v, want %v", *got, *tt.wantDecision)
			}

			if tt.wantErr {
				var perr *scherror.PermanentError
				if !errors.As(err, &perr) {
					t.Fatalf("expected PermanentError, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
