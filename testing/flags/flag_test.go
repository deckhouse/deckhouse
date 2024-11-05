package flags

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		args            []string
		expectedVerbose bool
		wantErr         bool
	}{
		{
			nil,
			false,
			false,
		},
		{
			[]string{"-test.run", `^\QTestReleaseControllerTestSuite\E$/^\QTestCreateReconcile\E$/^\QAutoPatch\E$/^\Qpatch_update_respect_window\E$`},
			false,
			false,
		},
		{
			[]string{"-test.v", "-test.run", `some-invalid-regexp\`},
			true,
			true,
		},
		{
			[]string{"-not.defined", "-test.v"},
			false, // TODO: true
			false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Parse(%s)", tt.args), func(t *testing.T) {
			if err := Parse(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.expectedVerbose != Verbose {
				t.Fail()
			}
		})
	}
}
