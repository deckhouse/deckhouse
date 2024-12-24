// Copyright 2024 Flant JSC
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

package config

import (
	"fmt"
	"math/rand"
	"sigs.k8s.io/yaml"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/certificate"
	"golang.org/x/crypto/bcrypt"
)

const (
	RegistryModeDirect   = "Direct"
	RegistryModeInDirect = "InDirect"
	RegistryModeProxy    = "Proxy"
	RegistryModeDetached = "Detached"
)

type SystemRegistryConfig struct {
	Enable bool
}

func (cfg *SystemRegistryConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(cfg)
}

type RegistryAccessData struct {
	CA     certificate.Authority `json:"ca"`
	UserRw RegistryUser          `json:"userRw"`
	UserRo RegistryUser          `json:"userRo"`
}

type RegistryUser struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
}

func (d *RegistryAccessData) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"ca": map[string]interface{}{
			"cert": d.CA.Cert,
			"key":  d.CA.Key,
		},
		"userRw": map[string]interface{}{
			"name":         d.UserRw.Name,
			"password":     d.UserRw.Password,
			"passwordHash": d.UserRw.PasswordHash,
		},
		"userRo": map[string]interface{}{
			"name":         d.UserRo.Name,
			"password":     d.UserRo.Password,
			"passwordHash": d.UserRo.PasswordHash,
		},
	}
}

func getRegistryAccessData() (*RegistryAccessData, error) {
	registryAccessDataCacheName := "system-registry-access-data"
	registryUserRw := "registry-user-rw"
	registryUserRo := "registry-user-ro"

	inCache, err := cache.Global().InCache(registryAccessDataCacheName)
	if err != nil {
		return nil, err
	}
	if inCache {
		var registryAccessData RegistryAccessData
		err := cache.Global().LoadStruct(registryAccessDataCacheName, &registryAccessData)
		return &registryAccessData, err
	} else {
		authority, err := newRegistryAuthority()
		if err != nil {
			return nil, err
		}

		userRw, err := newRegistryUser(registryUserRw)
		if err != nil {
			return nil, err
		}

		userRo, err := newRegistryUser(registryUserRo)
		if err != nil {
			return nil, err
		}

		registryAccessData := RegistryAccessData{
			CA:     authority,
			UserRw: *userRw,
			UserRo: *userRo,
		}
		err = cache.Global().SaveStruct(registryAccessDataCacheName, registryAccessData)
		return &registryAccessData, err
	}
}

func newRegistryAuthority() (certificate.Authority, error) {
	return certificate.GenerateCA(
		"embedded-registry-ca",
	)
}

func newRegistryUser(name string) (*RegistryUser, error) {
	password := generateRegistryPassword()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("error generating the password hash for the user %s: %v", name, err)
	}

	registryUser := RegistryUser{Name: name, Password: password, PasswordHash: string(passwordHash)}
	return &registryUser, nil
}

func generateRegistryPassword() string {
	const passwordLength = 16
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Creating a new random number generator
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	password := make([]byte, passwordLength)
	for i := range password {
		password[i] = charset[rng.Intn(len(charset))]
	}
	return string(password)
}
