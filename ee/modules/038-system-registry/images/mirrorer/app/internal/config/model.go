/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"fmt"
	"io"
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"gopkg.in/yaml.v3"
)

type Config struct {
	CAFile          string   `yaml:"ca,omitempty"`
	Users           Users    `yaml:"users"`
	LocalAddress    string   `yaml:"local"`
	RemoteAddresses []string `yaml:"remote"`
	SleepInterval   int      `yaml:"sleep,omitempty"`
	Parallelizm     int      `yaml:"parallelizm,omitempty"`
}

func (config *Config) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Users, validation.Required),
		validation.Field(&config.LocalAddress, validation.Required),
		validation.Field(&config.RemoteAddresses, validation.Required),
	)
}

type Users struct {
	Puller UserInfo `yaml:"puller"`
	Pusher UserInfo `yaml:"pusher"`
}

func (u *Users) Validate() error {
	return validation.ValidateStruct(u,
		validation.Field(&u.Puller, validation.Required),
		validation.Field(&u.Pusher, validation.Required),
	)
}

type UserInfo struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

func (ui *UserInfo) Validate() error {
	return validation.ValidateStruct(ui,
		validation.Field(&ui.Name, validation.Required),
		validation.Field(&ui.Password, validation.Required),
	)
}

func FromFile(filePath string) (config Config, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		err = fmt.Errorf("failed to open file: %w", err)
		return
	}
	defer file.Close()

	config, err = parse(file)
	return
}

func parse(reader io.Reader) (config Config, err error) {
	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(&config)

	if err != nil {
		err = fmt.Errorf("failed to decode YAML: %w", err)
		return
	}

	return
}
