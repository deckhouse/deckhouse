// Copyright 2025 Flant JSC
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

package provider

func extractVMChecker(name string, settings map[string]any, ns, typ string) (string, isVMChecker, error) {
	vm, err := extractString("vmResourceType", settings, name)
	if err != nil {
		return "", nil, err
	}

	key := ns + "/" + typ

	if name == "kubernetes" {
		return key, dvpProviderVMChecker(), nil
	}

	return key, genericVMChecker(vm), nil
}
