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

package debugserver

import (
	"net/http"

	"github.com/flant/shell-operator/pkg/debug"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

// RegisterRoutes register routes for dumping requirements memory storage
func RegisterRoutes(dbgSrv *debug.Server) {
	dbgSrv.RegisterHandler(http.MethodGet, "/requirements", func(_ *http.Request) (interface{}, error) {
		return requirements.DumpValues(), nil
	})
}
