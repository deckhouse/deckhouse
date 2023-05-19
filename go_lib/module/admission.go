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

package module

import (
	"net/http"
	"path"
	"strings"
)

type AdmissionServer interface {
	RegisterHandler(route string, handler http.Handler)
}

var (
	routes = make(map[string]http.Handler)
)

func SetupAdmissionRoutes(srv AdmissionServer) {
	for route, handler := range routes {
		srv.RegisterHandler(route, handler)
	}
}

func RegisterValidationHandler(route string, handler http.Handler) {
	if !strings.HasPrefix(route, "/validate") {
		route = path.Join("/validate", route)
	}

	routes[route] = handler
}
