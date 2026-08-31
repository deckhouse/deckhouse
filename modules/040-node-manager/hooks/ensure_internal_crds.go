/*
Copyright 2026 Flant JSC

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

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"

// The Cluster API bootstrap objects live under crds/internal because nobody
// creates them by hand: a MachineSet clones them and node-controller renders the
// machine's configuration into them. A subdirectory keeps them out of the
// documentation, which collects every CRD it finds, and out of the automatic
// install, which only takes the root of crds/ — hence this hook.
var _ = ensure_crds.RegisterEnsureCRDsHook("/deckhouse/modules/040-node-manager/crds/internal/*.yaml")
