// Copyright 2025 Flant JSC
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

package version

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
)

const template = `
terraform {
  required_version = ">= 0.14.8"
  required_providers {
    %s = {
      source  = "%s"
      version = ">= %s"
    }
  }
}
`

func GetVersionContent(settings settings.ProviderSettings, version string) []byte {
	source := settings.Namespace() + "/" + settings.Type()
	return []byte(fmt.Sprintf(template, settings.Type(), source, version))
}
