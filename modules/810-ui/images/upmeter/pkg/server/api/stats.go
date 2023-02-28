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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
)

type StatsHandler struct {
	DbCtx *dbcontext.DbContext
}

// TODO make better stats!
func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("Stats", r.RemoteAddr, r.RequestURI)

	stats, err := getStats(h.DbCtx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	out, err := json.Marshal(stats)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func getStats(dbctx *dbcontext.DbContext) ([]string, error) {
	daoCtx := dbctx.Start()
	defer daoCtx.Stop()

	dao30s := dao.NewEpisodeDao30s(daoCtx)
	return dao30s.Stats()
}
