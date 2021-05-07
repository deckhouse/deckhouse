package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/server/entity"
)

type ProbeListHandler struct {
	DbCtx *dbcontext.DbContext
}

func (h *ProbeListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("ProbeList", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	probeRefs, err := getRefs(h.DbCtx)

	out, err := json.Marshal(probeRefs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(out)
}

// getRefs selects group and probe from episodes
func getRefs(dbctx *dbcontext.DbContext) ([]check.ProbeRef, error) {
	daoCtx := dbctx.Start()
	defer daoCtx.Stop()

	dao5m := dao.NewEpisodeDao5m(daoCtx)
	probeRefs, err := dao5m.ListGroupProbe()
	if err != nil {
		return nil, err
	}

	probeRefs = entity.FilterDisabledProbesFromGroupProbeList(probeRefs)
	return probeRefs, nil
}
