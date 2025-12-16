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

type Config struct {
	CA *CertKey `json:"ca,omitempty" yaml:"ca,omitempty"`
}

func (c Config) ToMap() map[string]any {
	result := make(map[string]any)

	if c.CA != nil {
		if c.CA.Cert != "" || c.CA.Key != "" {
			caMap := make(map[string]any)

			if c.CA.Cert != "" {
				caMap["cert"] = c.CA.Cert
			}
			if c.CA.Key != "" {
				caMap["key"] = c.CA.Key
			}

			result["ca"] = caMap
		}
	}
	return result
}
