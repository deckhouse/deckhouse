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
	log.Println("Stats", r.RemoteAddr, r.RequestURI)

	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	dao30s := dao.NewDowntime30sDao(daoCtx)
	stats, err := dao30s.Stats()
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
