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
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bootstrap"
)

type User = users.User

type Params struct {
	RO bool `json:"ro,omitempty"`
}

func (params Params) Any() bool {
	return params.RO
}

type Inputs map[string]User

type State struct {
	RO           *User `json:"ro,omitempty"`
	RW           *User `json:"rw,omitempty"`
	MirrorPuller *User `json:"mirror_puller,omitempty"`
	MirrorPusher *User `json:"mirror_pusher,omitempty"`
}

func (state *State) GetParams() Params {
	return Params{
		RO: state.RO != nil,
	}
}

func (state *State) Process(params Params, inputs Inputs, bootstrap bootstrap.Inputs) error {
	if bootstrap.IsActive {
		state.RO = &bootstrap.Config.UserRO
	}

	if params.RO {
		if user, err := processUser("ro", state.RO, inputs); err == nil {
			state.RO = &user
		} else {
			return fmt.Errorf("cannot process ro user: %w", err)
		}
	} else {
		state.RO = nil
	}

	return nil
}

func processUser(name string, state *User, inputs Inputs) (User, error) {
	var user User

	if state.IsValid() {
		user = *state
	} else if inputUser, ok := inputs[SecretName(name)]; ok && inputUser.IsValid() {
		user = inputUser
	} else {
		user = User{
			UserName: name,
		}

		if err := user.GenerateNewPassword(); err != nil {
			return user, fmt.Errorf("cannot generate user \"%v\" password: %w", name, err)
		}
	}

	if !user.IsPasswordHashValid() {
		if err := user.UpdatePasswordHash(); err != nil {
			return user, fmt.Errorf("cannot update user \"%v\" password hash: %w", name, err)
		}
	}

	return user, nil
}
