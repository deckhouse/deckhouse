package module

import (
	"fmt"
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
	fmt.Println("D8 ROUTAES", routes)
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
