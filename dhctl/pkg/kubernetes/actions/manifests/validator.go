// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manifests

import (
	"errors"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
)

const maxLenghtSecretName = 63

func SecretNameLenghtValidator(manifest interface{}) error {
	data, ok := manifest.(*apiv1.Secret)
	if !ok {
		return errors.New("manifest is not *apiv1.Secret")
	}

	if len(data.ObjectMeta.Name) > maxLenghtSecretName {
		return fmt.Errorf("the length of the secret name %s must be less than %d",
			data.ObjectMeta.Name,
			maxLenghtSecretName,
		)
	}

	return nil
}
