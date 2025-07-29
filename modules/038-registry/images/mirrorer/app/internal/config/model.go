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

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"sigs.k8s.io/yaml"
)

type Config struct {
	CAFile          string   `json:"ca,omitempty"`
	Users           Users    `json:"users"`
	LocalAddress    string   `json:"local"`
	RemoteAddresses []string `json:"remote"`
	SleepInterval   int      `json:"sleep,omitempty"`
	Parallelizm     int      `json:"parallelizm,omitempty"`
}

func (config *Config) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Users, validation.Required),
		validation.Field(&config.LocalAddress, validation.Required),
		validation.Field(&config.RemoteAddresses, validation.Required),
	)
}

type Users struct {
	Puller UserInfo `json:"puller"`
	Pusher UserInfo `json:"pusher"`
}

func (u *Users) Validate() error {
	return validation.ValidateStruct(u,
		validation.Field(&u.Puller, validation.Required),
		validation.Field(&u.Pusher, validation.Required),
	)
}

type UserInfo struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (ui *UserInfo) Validate() error {
	return validation.ValidateStruct(ui,
		validation.Field(&ui.Name, validation.Required),
		validation.Field(&ui.Password, validation.Required),
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
