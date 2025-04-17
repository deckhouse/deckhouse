/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package models

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
)

type UserModel struct {
	Name     string
	Password string
}

func (u *UserModel) Auth() string {
	authRaw := fmt.Sprintf("%s:%s", u.Name, u.Password)
	return base64.StdEncoding.EncodeToString([]byte(authRaw))
}

func InputValuesToUserModel(input *go_hook.HookInput, objLocation string) (*UserModel, error) {
	var ret UserModel
	err := helpers.UnmarshalInputValue(input, objLocation, &ret)
	if errors.Is(err, helpers.ErrInputValueNotExist) {
		return nil, nil
	}
	return &ret, err
}

func FilterUserSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("failed to convert registry user secret to struct: %w", err)
	}

	ret := UserModel{
		Name:     string(secret.Data["name"]),
		Password: string(secret.Data["password"]),
	}
	return ret, nil
}

func ExtractFromSnapUserModel(snaps []go_hook.FilterResult) *UserModel {
	if len(snaps) == 0 {
		return nil
	}
	if snaps[0] == nil {
		return nil
	}
	ret := snaps[0].(UserModel)
	return &ret
}
