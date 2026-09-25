// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestChecker_CheckDeckhouseUser(t *testing.T) {
	tests := []struct {
		name          string
		executeError  error
		executeOutput []byte
		expectedError string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockScript)
	}{
		{
			name:          "deckhouse user check successful",
			executeError:  nil,
			executeOutput: []byte("deckhouse user and group are not present\n"),
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte("deckhouse user and group are not present\n"), nil)
			},
		},
		{
			name:          "deckhouse user check failed with exit error",
			executeError:  &exec.ExitError{Stderr: []byte("deckhouse user exists")},
			executeOutput: []byte("User check failed"),
			// The host used to be appended to the script's own sentence; it is the subject of the
			// failure now and lives in Checked, so the message is what the node said.
			expectedError: "User check failed",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				exitErr := &exec.ExitError{Stderr: []byte("deckhouse user exists")}
				msc.On("Execute", mock.Anything).Return([]byte("User check failed"), exitErr)
			},
		},
		{
			name:          "generic execution error",
			executeError:  errors.New("network error"),
			executeOutput: []byte(""),
			expectedError: "cannot check the deckhouse user and group on ",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte(""), errors.New("network error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockScript := &mocks.MockScript{}
			tt.setupMock(mockNode, mockScript)

			check := DeckhouseUserCheck{
				NodeInterface: FixedNodeInterface(mockNode),
				globalOptions: candiOptionsFor(t, "check_deckhouse_user.sh.tpl"),
			}
			err := check.Run(t.Context())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockScript.AssertExpectations(t)
		})
	}
}

// A leftover deckhouse account is the usual reason a node that was in a cluster before will not
// join a new one, and the operator is the one who has to remove it. The old message was
// "Deckhouse user existence check failed: execute on remote: exit status 1", repeated five times
// by a retry loop — it named neither what was found nor what to do, and the script's own
// diagnosis was thrown away before anyone saw it.
func TestDeckhouseUserFailureTellsTheOperatorWhatToRun(t *testing.T) {
	node := newFakeNode().onScript("check_deckhouse_user.sh").
		printsAndExits("deckhouse user or group exists with unexpected id: uid=1001, gid=1001 (expected 64535)", 1)

	check := DeckhouseUserCheck{
		NodeInterface: FixedNodeInterface(node),
		globalOptions: candiOptionsFor(t, "check_deckhouse_user.sh.tpl"),
	}

	err := check.Run(t.Context())

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "uid=1001", "what the node found is the node's to say")
	assert.Contains(t, failure.Expected, "64535")
	assert.Contains(t, failure.Fix, "cleanup_static_node.sh",
		"the cleanup script's path is the one step an operator cannot guess")
	assert.Contains(t, failure.Fix, "userdel deckhouse")

	// A leftover account does not remove itself between attempts.
	assert.True(t, isPermanent(err))
}
