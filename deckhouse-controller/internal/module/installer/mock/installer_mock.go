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

package mock

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type Installer struct {
}

func (i *Installer) Install(_ context.Context, _, _, _ string) error {
	return nil
}

func (i *Installer) Uninstall(_ context.Context, _ string) error {
	return nil
}

func (i *Installer) Download(_ context.Context, _ *v1alpha1.ModuleSource, _ string, _ string) (string, error) {
	return "testdata//validation/module", nil
}
