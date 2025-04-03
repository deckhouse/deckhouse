/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package models

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
)

type CertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

func InputValuesToCertModel(input *go_hook.HookInput, objLocation string) (*CertModel, error) {
	var ret CertModel
	err := helpers.UnmarshalInputValue(input, objLocation, &ret)
	if errors.Is(err, helpers.InputValueNotExist) {
		return nil, nil
	}
	return &ret, err
}

func FilterCertModelSecret(key string) go_hook.FilterFunc {
	return func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
		var secret v1core.Secret

		err := sdk.FromUnstructured(obj, &secret)
		if err != nil {
			return nil, fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
		}

		ret := CertModel{
			Cert: string(secret.Data[fmt.Sprintf("%s.crt", key)]),
			Key:  string(secret.Data[fmt.Sprintf("%s.key", key)]),
		}
		return ret, nil
	}
}

func ExtractFromSnapCertModel(snaps []go_hook.FilterResult) *CertModel {
	if len(snaps) == 0 {
		return nil
	}
	if snaps[0] == nil {
		return nil
	}
	ret := snaps[0].(CertModel)
	return &ret
}
