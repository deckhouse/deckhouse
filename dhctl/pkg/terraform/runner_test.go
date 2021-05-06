package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
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

func newTestRunnerWithChanges() *Runner {
	r := NewRunner("a", "b", "c", "d")
	r.changesInPlan = PlanHasChanges
	return r
}

func TestCheckRunnerHandleChanges(t *testing.T) {
	tests := []struct {
		name   string
		runner *Runner
		skip   bool
		err    error
	}{
		{
			name: "Yes and skip must not skip",
			skip: false,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithSkipChangesOnDeny(true).
				WithConfirm(func() *input.Confirmation {
					return input.NewConfirmation().WithYesByDefault()
				}),
		},
		{
			name: "Yes without skip must not skip",
			skip: false,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithConfirm(func() *input.Confirmation {
					return input.NewConfirmation().WithYesByDefault()
				}),
		},
		{
			name: "No and skip must skip",
			skip: true,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithSkipChangesOnDeny(true),
		},
		{
			name:   "No without skip must throw an error",
			skip:   false,
			err:    ErrTerraformApplyAborted,
			runner: newTestRunnerWithChanges(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skip, err := tc.runner.handleChanges()
			require.Equal(t, tc.skip, skip)
			if tc.err != nil {
				require.Error(t, err)
				require.EqualError(t, tc.err, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
