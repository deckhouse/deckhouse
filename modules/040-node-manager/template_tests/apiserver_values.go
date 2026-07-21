/*
Copyright 2021 Flant JSC

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

package template_tests

import (
	"github.com/deckhouse/deckhouse/testing/helm"
)

// setBashibleAPIServerTLSValues is a no-op: node-controller (the bashible-apiserver-cert
// controller) now owns the bashible-api-server-tls Secret and injects the APIService
// caBundle at runtime, so helm no longer consumes any bashibleApiServer* values.
func setBashibleAPIServerTLSValues(_ *helm.Config) {}
