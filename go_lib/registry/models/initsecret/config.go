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

type CertKey struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

func (c CertKey) ToMap() map[string]any {
	m := make(map[string]any)

	if c.Cert != "" {
		m["cert"] = c.Cert
	}
	if c.Key != "" {
		m["key"] = c.Key
	}

	if len(m) == 0 {
		return nil
	}
	return m
}

type User struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	PasswordHash string `json:"password_hash"`
}

func (u User) ToMap() map[string]any {
	m := make(map[string]any)

	if u.Name != "" {
		m["name"] = u.Name
	}
	if u.Password != "" {
		m["password"] = u.Password
	}
	if u.PasswordHash != "" {
		m["password_hash"] = u.PasswordHash
	}

	if len(m) == 0 {
		return nil
	}
	return m
}

type Config struct {
	CA     *CertKey `json:"ca,omitempty" yaml:"ca,omitempty"`
	ROUser *User    `json:"ro_user,omitempty" yaml:"ro_user,omitempty"`
	RWUser *User    `json:"rw_user,omitempty" yaml:"rw_user,omitempty"`
}

func (c Config) ToMap() map[string]any {
	result := make(map[string]any)

	if c.CA != nil {
		if ca := c.CA.ToMap(); ca != nil {
			result["ca"] = ca
		}
	}

	if c.ROUser != nil {
		if ro := c.ROUser.ToMap(); ro != nil {
			result["ro_user"] = ro
		}
	}

	if c.RWUser != nil {
		if rw := c.RWUser.ToMap(); rw != nil {
			result["rw_user"] = rw
		}
	}

	return result
}
