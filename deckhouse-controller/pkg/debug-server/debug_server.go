package debugserver

import (
	"net/http"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/flant/shell-operator/pkg/debug"
)

// RegisterRoutes register routes for dumping requirements memory storage
func RegisterRoutes(dbgSrv *debug.Server) {
	dbgSrv.RegisterHandler(http.MethodGet, "/requirements", func(req *http.Request) (interface{}, error) {
		return requirements.DumpValues().(map[string]interface{}), nil
	})
}
