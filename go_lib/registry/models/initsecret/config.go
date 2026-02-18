/*
Copyright 2025 Flant JSC

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

package initsecret

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type CertKey struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

func (c CertKey) ToMap() map[string]any {
	m := make(map[string]any)

	m["cert"] = c.Cert
	m["key"] = c.Key
	return m
}

func (c CertKey) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Cert, validation.Required),
		validation.Field(&c.Key, validation.Required),
	)
}

type User struct {
	Name         string `json:"name" yaml:"name"`
	Password     string `json:"password" yaml:"password"`
	PasswordHash string `json:"password_hash" yaml:"password_hash"`
}

func (u User) ToMap() map[string]any {
	m := make(map[string]any)

	m["name"] = u.Name
	m["password"] = u.Password
	m["password_hash"] = u.PasswordHash
	return m
}

func (u User) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Name, validation.Required),
		validation.Field(&u.Password, validation.Required),
		validation.Field(&u.PasswordHash, validation.Required),
	)
}

type Config struct {
	CA     CertKey `json:"ca" yaml:"ca"`
	ROUser User    `json:"ro_user" yaml:"ro_user"`
	RWUser User    `json:"rw_user" yaml:"rw_user"`
}

func (c Config) ToMap() map[string]any {
	m := make(map[string]any)

	m["ca"] = c.CA.ToMap()
	m["ro_user"] = c.ROUser.ToMap()
	m["rw_user"] = c.RWUser.ToMap()
	return m
}

func (c Config) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.CA, validation.Required),
		validation.Field(&c.ROUser, validation.Required),
		validation.Field(&c.RWUser, validation.Required),
	)
}
