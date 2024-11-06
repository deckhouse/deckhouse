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

package docs

type Version struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

func (svc *Service) List() (versions []Version, err error) {
	cm := svc.channelMappingEditor.get()

	for moduleName := range cm {
		for channels := range cm[moduleName] {
			for _, entity := range cm[moduleName][channels] {
				versions = append(versions, Version{
					Module:  moduleName,
					Version: entity.Version,
				})
			}
		}
	}

	return versions, nil
}
