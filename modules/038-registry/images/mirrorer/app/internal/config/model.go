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
	// These assertions are the reason the nested rules below run at all. Users
	// and UserInfo appear as struct fields by value, so ozzo only reaches their
	// Validate through the Validatable interface -- and a pointer receiver would
	// leave the value type not implementing it, silently skipping every rule
	// inside them.
	_ validation.Validatable = Config{}
	_ validation.Validatable = Users{}
	_ validation.Validatable = UserInfo{}
)

type Config struct {
	CAFile          string   `json:"ca,omitempty"`
	Users           Users    `json:"users"`
	LocalAddress    string   `json:"local"`
	RemoteAddresses []string `json:"remote"`
	SleepInterval   int      `json:"sleep,omitempty"`
	Parallelizm     int      `json:"parallelizm,omitempty"`
}

func (config Config) Validate() error {
	return validation.ValidateStruct(&config,
		validation.Field(&config.Users, validation.Required),
		// The addresses are handed to name.NewRegistry when the mirrorer is
		// constructed. Checking them here means a bad one is reported as a
		// configuration error at startup rather than after validation has
		// already passed.
		validation.Field(&config.LocalAddress, validation.Required, validation.By(registryAddress)),
		validation.Field(&config.RemoteAddresses,
			validation.Required,
			validation.Each(validation.Required, validation.By(registryAddress)),
		),
		// A negative limit is not a small limit: errgroup.SetLimit reads it as
		// no limit at all, which would have the mirrorer pull from every
		// repository at once against the registry the whole cluster pulls from.
		validation.Field(&config.Parallelizm, validation.Min(0)),
		validation.Field(&config.SleepInterval, validation.Min(0)),
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
// to a replica, so a line break in one of them would split that request.
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

type Users struct {
	Puller UserInfo `json:"puller"`
	Pusher UserInfo `json:"pusher"`
}

func (u Users) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Puller, validation.Required),
		validation.Field(&u.Pusher, validation.Required),
	)
}

type UserInfo struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (ui UserInfo) Validate() error {
	return validation.ValidateStruct(&ui,
		validation.Field(&ui.Name, validation.Required, validation.By(headerSafeString)),
		validation.Field(&ui.Password, validation.Required, validation.By(headerSafeString)),
	)
}

func FromFile(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		err = fmt.Errorf("failed to open file: %w", err)
		return Config{}, err
	}
	defer file.Close()

	return parse(file)
}

func parse(reader io.Reader) (Config, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		return config, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return config, nil
}
