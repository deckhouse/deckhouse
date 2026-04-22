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

package config

import (
	"fmt"
	"io"
	"os"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Src  Registry `json:"source"`
	Dest Registry `json:"destination"`
}

func (c *Config) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.Src, validation.Required),
		validation.Field(&c.Dest, validation.Required),
	)
}

type Registry struct {
	Address string `json:"address"`
	User    *User  `json:"user,omitempty"`
	CA      string `json:"ca,omitempty"`
}

func (r *Registry) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.Address, validation.Required),
		validation.Field(&r.User),
	)
}

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (u *User) Validate() error {
	return validation.ValidateStruct(u,
		validation.Field(&u.Name, validation.Required),
		validation.Field(&u.Password, validation.Required),
	)
}

func FromFile(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		err = fmt.Errorf("open file: %w", err)
		return Config{}, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return Config{}, fmt.Errorf("read file: %w", err)
	}

	return FromBytes(content)
}

func FromBytes(content []byte) (Config, error) {
	var config Config

	err := yaml.Unmarshal(content, &config)
	if err != nil {
		return config, fmt.Errorf("decode YAML: %w", err)
	}
	return config, nil
}
