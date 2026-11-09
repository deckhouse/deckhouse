/*
Copyright 2026 Flant JSC

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

package bashible

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
)

var (
	_ validation.Validatable = ContextBootstrap{}
)

type ContextBootstrap struct {
	Init initsecret.Config `json:"init" yaml:"init"`
}

func (c ContextBootstrap) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Init, validation.Required),
	)
}

func (c ContextBootstrap) ToMap() map[string]any {
	m := make(map[string]any)

	m["init"] = c.Init.ToMap()

	return m
}
