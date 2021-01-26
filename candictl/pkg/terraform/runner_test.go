package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPlanDestructiveChanges(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		destructive bool
		err         error
	}{
		/*{
			name:        "No Changes",
			path:        "./mock/no_changes.tfplan",
			destructive: false,
			err:         nil,
		},
		{
			name:        "Has changes",
			path:        "./mock/has_changes.tfplan",
			destructive: true,
			err:         nil,
		},*/
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, err := checkPlanDestructiveChanges(tc.path)
			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.destructive, code)
		})
	}
}
