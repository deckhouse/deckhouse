/*
Copyright 2023 Flant JSC

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

package v1alpha1

// +k8s:deepcopy-gen=package

//go:generate deepcopy-gen --input-dirs github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis/v1alpha1 -O zz_generated.deepcopy --go-header-file ./boilerplate.go.txt -o /tmp
//go:generate cp /tmp/github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis/v1alpha1/zz_generated.deepcopy.go ./
