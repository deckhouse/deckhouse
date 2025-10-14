/*
Copyright 2021 Flant JSC

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

package certificate

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

func ApplyCaSelfSignedCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned ca secret to secret: %v", err)
	}

	return Authority{
		Key:  string(secret.Data["tls.key"]),
		Cert: string(secret.Data["tls.crt"]),
	}, nil
}

func GetOrCreateCa(input *go_hook.HookInput, snapshot, cn string) (*Authority, error) {
	caList, err := sdkobjectpatch.UnmarshalToStruct[Authority](input.Snapshots, snapshot)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %q snapshot: %w", snapshot, err)
	}

	if len(caList) == 1 {
		return &caList[0], nil
	}

	selfSignedCA, err := GenerateCA(input.Logger, cn)
	if err != nil {
		return nil, fmt.Errorf("cannot generate self-signed CA: %w", err)
	}

	return &selfSignedCA, nil
}
