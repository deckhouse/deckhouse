/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

type User = users.User

type Params struct {
	RO       bool `json:"ro,omitempty"`
	RW       bool `json:"rw,omitempty"`
	Mirrorer bool `json:"mirrorer,omitempty"`
}

func (params Params) Any() bool {
	return params.RO || params.RW || params.Mirrorer
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
		RO:       state.RO != nil,
		RW:       state.RW != nil,
		Mirrorer: state.MirrorPuller != nil || state.MirrorPusher != nil,
	}
}

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
