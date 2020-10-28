package executor

import (
	"context"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/agent/manager"
	"upmeter/pkg/probe/types"
)

const ExportGranularity = 30

type ProbeExecutor struct {
	ProbeManager     *manager.ProbeManager
	MetricStorage    *metric_storage.MetricStorage
	KubernetesClient kube.KubernetesClient

	LastExportTimestamp int64
	LastScrapeTimestamp int64

	ResultCh chan types.ProbeResult

	Results map[string]*types.ProbeResult

	ScrapeResults map[string]*types.DowntimeEpisode

	DowntimeEpisodesCh chan []types.DowntimeEpisode

	ctx    context.Context
	cancel context.CancelFunc
}

func NewProbeExecutor(ctx context.Context) *ProbeExecutor {
	p := &ProbeExecutor{
		ResultCh: make(chan types.ProbeResult),
		Results:  make(map[string]*types.ProbeResult),
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	return p
}

func (p *ProbeExecutor) WithProbeManager(mgr *manager.ProbeManager) {
	p.ProbeManager = mgr
}

func (p *ProbeExecutor) WithResultCh(ch chan types.ProbeResult) {
	p.ResultCh = ch
}

func (p *ProbeExecutor) WithDowntimeEpisodesCh(ch chan []types.DowntimeEpisode) {
	p.DowntimeEpisodesCh = ch
}

func (p *ProbeExecutor) WithKubernetesClient(client kube.KubernetesClient) {
	p.KubernetesClient = client
}

func (e *ProbeExecutor) Start() {
	// Set result chan for each probe.
	e.ProbeManager.InitProbes(e.ResultCh, e.KubernetesClient)

	// Probe restarter
	go func() {
		restartTick := time.NewTicker(time.Second)

		for {
			select {
			case <-e.ctx.Done():
				restartTick.Stop()
				// TODO stop probes
				// TODO signal to main
				return
			case <-restartTick.C:
				e.RestartProbes()
			}
		}
	}()

	// Scraper
	// Synced read/write of e.Results and e.ScrapeResults
	go func() {
		scrapeTick := time.NewTicker(time.Second)
		for {
			select {
			case <-e.ctx.Done():
				scrapeTick.Stop()
				return
			case <-scrapeTick.C:
				e.Scrape()
			case probeResult := <-e.ResultCh:
				log.Debugf("probe '%s' result %+v", probeResult.ProbeRef.ProbeId(), probeResult.CheckResults)
				storedResult, ok := e.Results[probeResult.ProbeRef.ProbeId()]
				if !ok {
					storedResult = &types.ProbeResult{
						ProbeRef: probeResult.ProbeRef,
					}
					e.Results[probeResult.ProbeRef.ProbeId()] = storedResult
				}
				storedResult.MergeChecks(probeResult)
			}
		}
	}()
}

func (e *ProbeExecutor) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

// RestartProbes checks if probe is running and restart them.
func (e *ProbeExecutor) RestartProbes() {
	//log.Infof("RestartProbes")
	currentSecond := time.Now().Unix()
	for _, prob := range e.ProbeManager.Probers() {
		if !prob.State().ShouldRun(currentSecond) {
			continue
		}

		// Run probe again
		//log.Infof("executor Start probe '%s' at %d", prob.Metadata().String(), currentSecond)
		_ = prob.Run(currentSecond)

		// Increase probe running counter
		e.MetricStorage.CounterAdd("upmeter_agent_probe_run_total",
			1.0, map[string]string{"probe": prob.ProbeId()})
	}
}

// Scrape checks probe results
func (e *ProbeExecutor) Scrape() {
	now := time.Now().Unix()
	timeslot := (now / 30) * 30
	// Scrape is started every second, but we may lose seconds
	// if timer is not precise.
	var delta int64 = 1
	var noDataDelta int64 = 0
	if e.LastScrapeTimestamp > 0 {
		delta = now - e.LastScrapeTimestamp
		e.LastScrapeTimestamp = now
	} else {
		// proper NoData for first 30 sec episode at start.
		noDataDelta = now - timeslot
	}

	if e.ScrapeResults == nil {
		e.ScrapeResults = make(map[string]*types.DowntimeEpisode)
	}

	for probeRefId, result := range e.Results {
		//log.Infof("Scrape check result: %+v", result)
		downtime, ok := e.ScrapeResults[probeRefId]
		if !ok {
			downtime = &types.DowntimeEpisode{
				ProbeRef: result.ProbeRef,
				TimeSlot: timeslot,
				NoData:   30,
			}
			e.ScrapeResults[probeRefId] = downtime
		}

		switch result.Value() {
		case 0:
			downtime.FailSeconds += delta
		case 1:
			downtime.SuccessSeconds += delta
		case 2:
			downtime.Unknown += delta
		}
		if noDataDelta > 0 {
			downtime.NoData -= noDataDelta
		} else {
			downtime.NoData -= delta
		}
		downtime.Correct(30)

		// Log some asserts
		if downtime.FailSeconds > ExportGranularity {
			log.Warnf("Probe '%s' has fail seconds %d that is more than export granularity %d", probeRefId, downtime.FailSeconds, ExportGranularity)
		}
		if downtime.SuccessSeconds > ExportGranularity {
			log.Warnf("Probe '%s' has success seconds %d that is more than export granularity %d", probeRefId, downtime.FailSeconds, ExportGranularity)
		}
	}

	// Send to sender every 30 seconds.
	shouldExport := e.CheckAndUpdateLastExportTime(now)
	if !shouldExport {
		return
	}

	// Copy scraped results and send to sender.
	exportResults := make([]types.DowntimeEpisode, 0)
	for _, downtime := range e.ScrapeResults {
		exportResults = append(exportResults, *downtime)
	}
	e.DowntimeEpisodesCh <- exportResults
	e.ScrapeResults = nil
}

func (e *ProbeExecutor) CheckAndUpdateLastExportTime(nowTime int64) bool {
	var shouldExport = false
	if e.LastExportTimestamp == 0 {
		// Export at start only if now is a 30 second mark
		if nowTime%ExportGranularity == 0 {
			shouldExport = true
		} else {
			// Set LastExportTimestamp to a prevMark for future calls
			e.LastExportTimestamp = (nowTime / ExportGranularity) * ExportGranularity
		}
	} else {
		prevMark := (e.LastExportTimestamp / ExportGranularity) * ExportGranularity

		//Export if now is a 30 second mark or past it
		if nowTime >= prevMark+ExportGranularity {
			shouldExport = true
		}
	}
	if shouldExport {
		e.LastExportTimestamp = nowTime
	}

	return shouldExport
}
