package executor

import (
	"context"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/agent/manager"
	"upmeter/pkg/checks"
)

const ExportGranularity = 30

type ProbeExecutor struct {
	ProbeManager        *manager.ProbeManager
	MetricStorage       *metric_storage.MetricStorage
	KubernetesClient    kube.KubernetesClient
	serviceAccountToken string

	LastExportTimestamp int64
	LastScrapeTimestamp int64

	ResultCh chan checks.Result

	Results map[string]*checks.Result

	ScrapeResults map[string]*checks.DowntimeEpisode

	DowntimeEpisodesCh chan []checks.DowntimeEpisode

	ctx    context.Context
	cancel context.CancelFunc
}

func NewProbeExecutor(ctx context.Context) *ProbeExecutor {
	p := &ProbeExecutor{
		ResultCh: make(chan checks.Result),
		Results:  make(map[string]*checks.Result),
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	return p
}

func (p *ProbeExecutor) WithProbeManager(mgr *manager.ProbeManager) {
	p.ProbeManager = mgr
}

func (p *ProbeExecutor) WithResultCh(ch chan checks.Result) {
	p.ResultCh = ch
}

func (p *ProbeExecutor) WithDowntimeEpisodesCh(ch chan []checks.DowntimeEpisode) {
	p.DowntimeEpisodesCh = ch
}

func (p *ProbeExecutor) WithKubernetesClient(client kube.KubernetesClient) {
	p.KubernetesClient = client
}

func (p *ProbeExecutor) WithServiceAccountToken(token string) {
	p.serviceAccountToken = token
}

func (p *ProbeExecutor) Start() {
	// Set result chan for each probe.
	p.ProbeManager.InitProbes(p.ResultCh, p.KubernetesClient, p.serviceAccountToken)

	// Probe restarter
	go func() {
		// The minimal period to spawn probes
		restartTick := time.NewTicker(100 * time.Millisecond)

		for {
			select {
			case <-p.ctx.Done():
				restartTick.Stop()
				// TODO stop probes
				// TODO signal to main
				return
			case <-restartTick.C:
				p.RestartProbes()
			}
		}
	}()

	// Scraper
	// Synced read/write of p.Results and p.ScrapeResults
	go func() {
		scrapeTick := time.NewTicker(time.Second)
		for {
			select {
			case <-p.ctx.Done():
				scrapeTick.Stop()
				return
			case <-scrapeTick.C:
				p.Scrape()
			case probeResult := <-p.ResultCh:
				log.Debugf("probe '%s' result %+v", probeResult.ProbeRef.Id(), probeResult.CheckResults)
				storedResult, ok := p.Results[probeResult.ProbeRef.Id()]
				if !ok {
					storedResult = &checks.Result{
						ProbeRef: probeResult.ProbeRef,
					}
					p.Results[probeResult.ProbeRef.Id()] = storedResult
				}
				storedResult.MergeChecks(probeResult)
			}
		}
	}()
}

func (p *ProbeExecutor) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// RestartProbes checks if probe is running and restart them.
func (p *ProbeExecutor) RestartProbes() {
	now := time.Now()
	for _, prob := range p.ProbeManager.Probes() {
		if !prob.State().ShouldRun(now) {
			continue
		}

		// Run probe again
		_ = prob.Run(now)

		// Increase probe running counter
		p.MetricStorage.CounterAdd("upmeter_agent_probe_run_total",
			1.0, map[string]string{"probe": prob.Id()})
	}
}

// Scrape checks probe results
func (p *ProbeExecutor) Scrape() {
	now := time.Now().Unix()
	timeslot := (now / 30) * 30
	// Scrape is started every second, but we may lose seconds
	// if timer is not precise.
	var delta int64 = 1
	var noDataDelta int64 = 0
	if p.LastScrapeTimestamp > 0 {
		delta = now - p.LastScrapeTimestamp
		p.LastScrapeTimestamp = now
	} else {
		// proper NoData for first 30 sec episode at start.
		noDataDelta = now - timeslot
	}

	if p.ScrapeResults == nil {
		p.ScrapeResults = make(map[string]*checks.DowntimeEpisode)
	}

	for probeRefId, result := range p.Results {
		downtime, ok := p.ScrapeResults[probeRefId]
		if !ok {
			downtime = &checks.DowntimeEpisode{
				ProbeRef: result.ProbeRef,
				TimeSlot: timeslot,
				NoData:   30,
			}
			p.ScrapeResults[probeRefId] = downtime
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
	shouldExport := p.CheckAndUpdateLastExportTime(now)
	if !shouldExport {
		return
	}

	// Copy scraped results and send to sender.
	exportResults := make([]checks.DowntimeEpisode, 0)
	for _, downtime := range p.ScrapeResults {
		exportResults = append(exportResults, *downtime)
	}
	p.DowntimeEpisodesCh <- exportResults
	p.ScrapeResults = nil
}

func (p *ProbeExecutor) CheckAndUpdateLastExportTime(nowTime int64) bool {
	var shouldExport = false
	if p.LastExportTimestamp == 0 {
		// Export at start only if now is a 30 second mark
		if nowTime%ExportGranularity == 0 {
			shouldExport = true
		} else {
			// Set LastExportTimestamp to a prevMark for future calls
			p.LastExportTimestamp = (nowTime / ExportGranularity) * ExportGranularity
		}
	} else {
		prevMark := (p.LastExportTimestamp / ExportGranularity) * ExportGranularity

		// Export if now is a 30 second mark or past it
		if nowTime >= prevMark+ExportGranularity {
			shouldExport = true
		}
	}
	if shouldExport {
		p.LastExportTimestamp = nowTime
	}

	return shouldExport
}
