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

const exportIntervalSeconds = 30

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
	if p.ScrapeResults == nil {
		p.ScrapeResults = make(map[string]*checks.DowntimeEpisode)
	}

	now := time.Now().Unix()
	p.RecalcEpisodes(now)
	p.LastScrapeTimestamp = now

	// Send to sender every 30 seconds.
	shouldExport := p.UpdateLastExportTime(now)
	if !shouldExport {
		return
	}
	p.Export()

	p.ScrapeResults = nil
}

// Export copies scraped results and sends them to sender along as evaluates computed probes.
func (p *ProbeExecutor) Export() {
	episodes := make([]checks.DowntimeEpisode, 0)

	for _, ep := range p.ScrapeResults {
		episodes = append(episodes, *ep)
	}

	// FIXME this is incorrect. The calculation of a correct downtime episode from two other downtime episodes is
	//       impossible. Without original time points, we cannot know how these downtime episodes overlap.
	//
	//      For example, consider 2 similar downtime episodes that look like this {success: 15, fail: 15}.
	//      How do they overlap?
	//
	//		1. Edge case: 100% overlap
	//
	//		 	|    15s    |    15s    |
	//		1	|---fail----|--success--|	 50% downtime
	//		2	|---fail----|--success--|	 50% downtime
	//		result	|---fail----|--success--|	 50% downtime
	//
	//		2. Edge case: 0% overlap
	//
	//		 	|    15s    |    15s    |
	//		1	|--success--|---fail----|	 50% downtime
	//		2	|---fail----|--success--|	 50% downtime
	//		result	|---fail----|---fail----|	100% downtime
	//
	//      For now, calc.Calc method picks biggest fail of two episodes like they fully overlap in fail,
	//      in unknown, and in nodata intervals, and overlap in success by the remains.
	for _, calc := range p.ProbeManager.Calculators() {
		ep, err := calc.Calc(p.ScrapeResults, exportIntervalSeconds)
		if err != nil {
			log.Errorf("cannot calculate probe id=%s: %v", calc.Id(), err)
			continue
		}
		episodes = append(episodes, *ep)
	}

	p.DowntimeEpisodesCh <- episodes
}

func (p *ProbeExecutor) RecalcEpisodes(now int64) {
	/*
		FIXME workaround timer/`now` inaccuracy

		Scrape starts every second, but we may lose seconds because the timer is not precise and we just
		throw away the precision of `now`. In exported results, we can observe something like this:

			Success: 31m 27s     = 30m + 87s
			Nodata:     -87s

		 We seem to over-operate with delta, or correct incorrectly.
	*/
	timeslot := (now / 30) * 30
	var delta, noDataDelta int64
	if p.LastScrapeTimestamp == 0 {
		// proper NoData for first 30 sec episode at start. We take delta into account.
		delta = 1
		noDataDelta = now - timeslot - delta
	} else {
		delta = now - p.LastScrapeTimestamp
		noDataDelta = 0
	}

	for id, result := range p.Results {
		episode, ok := p.ScrapeResults[id]
		if !ok {
			episode = &checks.DowntimeEpisode{
				ProbeRef:      result.ProbeRef,
				TimeSlot:      timeslot,
				NoDataSeconds: 30,
			}
			p.ScrapeResults[id] = episode
		}

		// Move spent time to an acknowledged status
		episode.NoDataSeconds -= delta
		switch result.Value() {
		case checks.StatusFail:
			episode.FailSeconds += delta
		case checks.StatusSuccess:
			episode.SuccessSeconds += delta
		case checks.StatusUnknown:
			episode.UnknownSeconds += delta
		}

		// Correct possible inaccuracy
		episode.NoDataSeconds -= noDataDelta
		episode.Correct(30)

		// Log some asserts
		if episode.FailSeconds > exportIntervalSeconds {
			log.Warnf("Probe '%s' fail time %ds exceeds export interval %ds\n", id, episode.FailSeconds, exportIntervalSeconds)
		}
		if episode.SuccessSeconds > exportIntervalSeconds {
			log.Warnf("Probe '%s' success time %ds exceeds export interval %ds\n", id, episode.FailSeconds, exportIntervalSeconds)
		}
	}
}

func (p *ProbeExecutor) UpdateLastExportTime(now int64) bool {
	if p.LastExportTimestamp == 0 {
		// Export at start only if now is a 30 second mark
		if now%exportIntervalSeconds == 0 {
			p.LastExportTimestamp = now
			return true
		}

		// Set LastExportTimestamp to the interval start for future calls
		p.LastExportTimestamp = (now / exportIntervalSeconds) * exportIntervalSeconds
		return false
	}

	// Export if now is a 30 second mark or past it
	start := (p.LastExportTimestamp / exportIntervalSeconds) * exportIntervalSeconds
	end := start + exportIntervalSeconds
	if now >= end {
		p.LastExportTimestamp = now
		return true
	}

	return false
}
