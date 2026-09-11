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
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/yaml"
)

var (
	// These assertions are the reason the nested rules below run at all. Src and
	// Dest are Registry values, so ozzo reaches their Validate only through the
	// Validatable interface -- and with a pointer receiver the value type does
	// not implement it, which silently skipped every rule inside them,
	// `address` being required included.
	_ validation.Validatable = Config{}
	_ validation.Validatable = Registry{}
	_ validation.Validatable = User{}
)

type Config struct {
	Src  Registry `json:"source"`
	Dest Registry `json:"destination"`
}

func (c Config) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Src, validation.Required),
		validation.Field(&c.Dest, validation.Required),
	)
}

type Registry struct {
	Address string `json:"address"`
	User    *User  `json:"user,omitempty"`
	CA      string `json:"ca,omitempty"`
}

func (r Registry) Validate() error {
	return validation.ValidateStruct(&r,
		// The address is handed to name.NewRegistry when the syncer connects.
		// Checking it here reports a bad one as a configuration error rather
		// than after validation has already passed.
		validation.Field(&r.Address, validation.Required, validation.By(registryAddress)),
		validation.Field(&r.User),
	)
}

// registryAddress reports whether a value is an address the registry client can
// construct: a bare `<host>[:<port>]` authority, with no scheme and no path.
func registryAddress(value any) error {
	raw, ok := value.(string)
	if !ok {
		return fmt.Errorf("must be a string, got %T", value)
	}
	if raw == "" {
		return nil
	}
	if _, err := name.NewRegistry(raw); err != nil {
		return fmt.Errorf("is not a registry address: %w", err)
	}
	return nil
}

// headerSafeString rejects the characters that cannot travel in an HTTP header.
// The credentials below are sent in the Authorization header of every request
// to the source and destination registries.
func headerSafeString(value any) error {
	raw, ok := value.(string)
	if !ok {
		return fmt.Errorf("must be a string, got %T", value)
	}
	if i := strings.IndexAny(raw, "\r\n\x00"); i >= 0 {
		return fmt.Errorf("must not contain %q at offset %d", raw[i], i)
	}
	return nil
}

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (u User) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Name, validation.Required, validation.By(headerSafeString)),
		validation.Field(&u.Password, validation.Required, validation.By(headerSafeString)),
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
