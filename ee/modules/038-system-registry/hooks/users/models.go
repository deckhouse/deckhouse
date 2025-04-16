/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

type Params struct {
	RO       bool `json:"ro,omitempty"`
	RW       bool `json:"rw,omitempty"`
	Mirrorer bool `json:"mirrorer,omitempty"`
}

func (params Params) Any() bool {
	return params.RO && params.RW && params.Mirrorer
}

type State struct {
	RO           *users.User `json:"ro,omitempty"`
	RW           *users.User `json:"rw,omitempty"`
	MirrorPuller *users.User `json:"mirror-puller,omitempty"`
	MirrorPusher *users.User `json:"mirror-pusher,omitempty"`
}

type Inputs map[string]users.User

func (state *State) Process(params Params, inputs Inputs) error {
	if params.RO {
		if user, err := processUser("ro", state.RO, inputs); err == nil {
			state.RO = &user
		} else {
			return fmt.Errorf("cannot process ro user: %w", err)
		}
	} else {
		state.RO = nil
	}

	if params.RW {
		if user, err := processUser("rw", state.RW, inputs); err == nil {
			state.RW = &user
		} else {
			return fmt.Errorf("cannot process rw user: %w", err)
		}
	} else {
		state.RW = nil
	}

	if params.Mirrorer {
		if user, err := processUser("mirror-puller", state.MirrorPuller, inputs); err == nil {
			state.MirrorPuller = &user
		} else {
			return fmt.Errorf("cannot process mirror-puller user: %w", err)
		}

		if user, err := processUser("mirror-pusher", state.MirrorPusher, inputs); err == nil {
			state.MirrorPusher = &user
		} else {
			return fmt.Errorf("cannot process mirror-pusher user: %w", err)
		}
	} else {
		state.MirrorPuller = nil
		state.MirrorPusher = nil
	}

	return nil
}

func processUser(name string, state *users.User, inputs Inputs) (users.User, error) {
	var user users.User

	if state.IsValid() {
		user = *state
	} else if inputUser, ok := inputs[userSecretName(name)]; ok && inputUser.IsValid() {
		user = inputUser
	} else {
		user = users.User{
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
