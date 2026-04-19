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

package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessUser(t *testing.T) {
	t.Run("Create new user if not in state or inputs", func(t *testing.T) {
		state := &User{}
		inputs := make(Inputs)

		user, err := processUser("testuser", state, inputs)
		require.NoError(t, err)

		assert.Equal(t, "testuser", user.UserName)
		assert.NotEmpty(t, user.Password, "Password should be generated")
		assert.True(t, user.IsPasswordHashValid(), "Password hash should be valid")
	})

	t.Run("Load user from valid state", func(t *testing.T) {
		stateUser := User{
			UserName: "stateuser",
			Password: "password",
		}
		require.NoError(t, stateUser.UpdatePasswordHash())

		inputs := make(Inputs)
		user, err := processUser("stateuser", &stateUser, inputs)
		require.NoError(t, err)

		assert.Equal(t, stateUser, user, "Should load user from state")
	})

	t.Run("Load user from valid inputs if not in state", func(t *testing.T) {
		inputUser := User{
			UserName: "inputuser",
			Password: "password",
		}
		require.NoError(t, inputUser.UpdatePasswordHash())

		state := &User{}
		inputs := Inputs{
			SecretName("inputuser"): inputUser,
		}

		user, err := processUser("inputuser", state, inputs)
		require.NoError(t, err)

		assert.Equal(t, inputUser.UserName, user.UserName)
		// Password is not carried over from input, so we check the hash
		assert.Equal(t, inputUser.HashedPassword, user.HashedPassword)
	})

	t.Run("Update password hash if invalid", func(t *testing.T) {
		stateUser := User{
			UserName:       "userwithbadhash",
			Password:       "newpassword",
			HashedPassword: "badhash", // Invalid hash
		}

		inputs := make(Inputs)
		user, err := processUser("userwithbadhash", &stateUser, inputs)
		require.NoError(t, err)

		assert.NotEqual(t, "badhash", user.HashedPassword)
		assert.True(t, user.IsPasswordHashValid(), "Hash should be updated to a valid one")
	})

	t.Run("Generate password if state user is empty", func(t *testing.T) {
		stateUser := User{UserName: "emptyuser"} // No password
		inputs := make(Inputs)

		user, err := processUser("emptyuser", &stateUser, inputs)
		require.NoError(t, err)
		require.NotEmpty(t, user.Password)
		assert.True(t, user.IsPasswordHashValid())
	})
}
