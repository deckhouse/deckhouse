package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/dao"
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
