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

package utils

import (
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

const (
	SyncedPollPeriod = 100 * time.Millisecond
)

// GenerateRegistryOptions feteches settings from ModuleSource and generate registry options from them
func GenerateRegistryOptions(ms *v1alpha1.ModuleSource) []cr.Option {
	opts := make([]cr.Option, 0)
	if ms.Spec.Registry.DockerCFG != "" {
		opts = append(opts, cr.WithAuth(ms.Spec.Registry.DockerCFG))
	} else {
		opts = append(opts, cr.WithDisabledAuth())
	}

	if ms.Spec.Registry.CA != "" {
		opts = append(opts, cr.WithCA(ms.Spec.Registry.CA))
	}

	if ms.Spec.Registry.Scheme == "HTTP" {
		opts = append(opts, cr.WithInsecureSchema(true))
	}

	return opts
}
