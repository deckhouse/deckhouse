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

package inclusterproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestProcessUserPasswordHash(t *testing.T) {
	logger := log.NewLogger()

	type testCase struct {
		name            string
		user            users.User
		expectedHashed  bool
		expectedNewHash bool
	}

	tests := []testCase{
		{
			name: "valid password and hash", // -> nothing should happen
			user: users.User{
				UserName:       "user",
				Password:       "password",
				HashedPassword: "$2a$10$UajCvgsxk0bk1kkR8Dfhhuy.jkJXMx3rTUgOJEp3SZM/Z4ThQwn2C",
			},
			expectedHashed:  true,
			expectedNewHash: false,
		},
		{
			name: "not empty password and not empty invalid hash", // -> should replace hash
			user: users.User{
				UserName:       "user",
				Password:       "password",
				HashedPassword: "123",
			},
			expectedHashed:  true,
			expectedNewHash: true,
		},
		{
			name: "empty password and empty hash", // -> nothing should happen
			user: users.User{
				UserName:       "user",
				Password:       "",
				HashedPassword: "",
			},
			expectedHashed:  false,
			expectedNewHash: false,
		},
		{
			name: "not empty password and empty hash", // -> should hash new password
			user: users.User{
				UserName:       "user",
				Password:       "password",
				HashedPassword: "",
			},
			expectedHashed:  true,
			expectedNewHash: true,
		},
		{
			name: "empty password and not empty hash", // -> should clear existing hash
			user: users.User{
				UserName:       "user",
				Password:       "",
				HashedPassword: "123",
			},
			expectedHashed:  false,
			expectedNewHash: true,
		},
	}

	assertHashProcessing := func(t *testing.T, user *users.User, expectedHashed, expectedNewHash bool, context string) {
		t.Helper()

		prevHash := user.HashedPassword
		err := processUserPasswordHash(logger, user)

		require.NoError(t, err, "%s: expected no error", context)

		isHashed := user.HashedPassword != ""
		isNewHash := prevHash != user.HashedPassword

		assert.Equalf(t, expectedHashed, isHashed, "%s: expected hashed=%v, got %v", context, expectedHashed, isHashed)
		assert.Equalf(t, expectedNewHash, isNewHash, "%s: expected newHash=%v, got %v", context, expectedNewHash, isNewHash)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user := tt.user

			// First run: initial hash processing
			assertHashProcessing(t, &user, tt.expectedHashed, tt.expectedNewHash, "first run")

			// Second run: should be idempotent
			assertHashProcessing(t, &user, tt.expectedHashed, false, "second run")
		})
	}
}
