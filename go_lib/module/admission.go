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
