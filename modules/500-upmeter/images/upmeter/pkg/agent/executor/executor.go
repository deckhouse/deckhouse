package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/agent/manager"
	"upmeter/pkg/check"
)

const exportIntervalSeconds = 30
const schedulePeriod = 100 * time.Millisecond
const scrapePeriod = time.Second

type ProbeExecutor struct {
	probeManager *manager.Manager
	metrics      *metric_storage.MetricStorage

	// receiving results from checks
	recv    chan check.Result
	results map[string]*check.Result

	// sending episodes to upmeter server
	send     chan []check.DowntimeEpisode
	episodes map[string]*check.DowntimeEpisode

	lastScrapeTimestamp int64
	lastExportTimestamp int64

	ctx    context.Context
	cancel context.CancelFunc
}

func NewProbeExecutor(ctx context.Context, mgr *manager.Manager, send chan []check.DowntimeEpisode) *ProbeExecutor {
	p := &ProbeExecutor{
		recv:    make(chan check.Result),
		results: make(map[string]*check.Result),
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.probeManager = mgr
	p.send = send
	return p
}

func (e *ProbeExecutor) Start() {
	// Set result chan
	e.probeManager.SendTo(e.recv)

	// Checks scheduler
	go func() {
		// The minimal period to spawn runners
		restartTick := time.NewTicker(schedulePeriod)

		for {
			select {
			case <-e.ctx.Done():
				restartTick.Stop()
				// TODO stop probes
				// TODO signal to main
				return
			case <-restartTick.C:
				e.run()
			}
		}
	}()

	// Scraper
	// Synced read/write of e.results and e.episodes
	go func() {
		scrapeTick := time.NewTicker(scrapePeriod)
		for {
			select {
			case <-e.ctx.Done():
				scrapeTick.Stop()
				return
			case <-scrapeTick.C:
				e.scrape()
			case result := <-e.recv:
				id := result.ProbeRef.Id()

				log.Debugf("probe '%s' result %+v", id, result.CheckResults)

				storedResult, ok := e.results[id]
				if !ok {
					storedResult = &check.Result{
						ProbeRef: result.ProbeRef,
					}
					e.results[id] = storedResult
				}
				storedResult.SetCheckStatus(result)
			}
		}
	}()
}

func (e *ProbeExecutor) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

// run checks if probe is running and restart them.
func (e *ProbeExecutor) run() {
	// rounding lets us avoid inaccuracies in time comparison
	now := time.Now().Round(schedulePeriod)
	for _, runner := range e.probeManager.Runners() {
		if !runner.ShouldRun(now) {
			continue
		}

		runner.Run(now)

		// Increase probe running counter
		e.metrics.CounterAdd("upmeter_agent_probe_run_total",
			1.0, map[string]string{"probe": runner.Id()})
	}
}

// scrape checks probe results
func (e *ProbeExecutor) scrape() {
	if e.episodes == nil {
		e.episodes = make(map[string]*check.DowntimeEpisode)
	}

	// rounding fixes go ticker inaccuracy
	now := time.Now().Round(scrapePeriod).Unix()
	e.recalcEpisodes(now)
	e.lastScrapeTimestamp = now

	// Send to sender every 30 seconds.
	shouldExport := e.updateLastExportTime(now)
	if !shouldExport {
		return
	}
	e.export()

	e.episodes = nil
}

// export copies scraped results and sends them to sender along as evaluates computed probes.
func (e *ProbeExecutor) export() {
	episodes := make([]check.DowntimeEpisode, 0)

	for _, ep := range e.episodes {
		fmt.Println("exporting", ep.DumpString())
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
	for _, calc := range e.probeManager.Calculators() {
		ep, err := calc.Calc(e.episodes, exportIntervalSeconds)
		if err != nil {
			log.Errorf("cannot calculate probe id=%s: %v", calc.Id(), err)
			continue
		}
		fmt.Println("exporting", ep.DumpString())
		episodes = append(episodes, *ep)
	}

	e.send <- episodes
}

func (e *ProbeExecutor) recalcEpisodes(now int64) {
	timeslot := (now / 30) * 30

	// 1 second by contract, because scraping period is 1 second and downtime episode aggregates intervals
	// with seconds precision
	var delta int64 = 1

	var noDataDelta int64
	if e.lastScrapeTimestamp == 0 {
		// proper NoData for first 30 sec episode at start. We take delta into account because we always
		// spread data with it.
		noDataDelta = now - timeslot - delta
	}

	for id, result := range e.results {
		episode, ok := e.episodes[id]
		if !ok {
			episode = &check.DowntimeEpisode{
				ProbeRef:      result.ProbeRef,
				TimeSlot:      timeslot,
				NoDataSeconds: exportIntervalSeconds,
			}
			e.episodes[id] = episode
		}

		// Move spent time to an acknowledged status
		episode.NoDataSeconds -= delta
		switch result.Value() {
		case check.StatusFail:
			episode.FailSeconds += delta
		case check.StatusSuccess:
			episode.SuccessSeconds += delta
		case check.StatusUnknown:
			episode.UnknownSeconds += delta
		}

		// Correct possible inaccuracy
		episode.NoDataSeconds -= noDataDelta
		episode.Correct(exportIntervalSeconds)

		// Log some asserts
		if episode.FailSeconds > exportIntervalSeconds {
			log.Warnf("Probe '%s' fail time %ds exceeds export interval %ds\n", id, episode.FailSeconds, exportIntervalSeconds)
		}
		if episode.SuccessSeconds > exportIntervalSeconds {
			log.Warnf("Probe '%s' success time %ds exceeds export interval %ds\n", id, episode.FailSeconds, exportIntervalSeconds)
		}
	}
}

func (e *ProbeExecutor) updateLastExportTime(now int64) bool {
	if e.lastExportTimestamp == 0 {
		// Export at start only if now is a 30 second mark
		if now%exportIntervalSeconds == 0 {
			e.lastExportTimestamp = now
			return true
		}

		// Set lastExportTimestamp to the interval start for future calls
		e.lastExportTimestamp = (now / exportIntervalSeconds) * exportIntervalSeconds
		return false
	}

	// Export if now is a 30 second mark or past it
	start := (e.lastExportTimestamp / exportIntervalSeconds) * exportIntervalSeconds
	end := start + exportIntervalSeconds
	if now >= end {
		e.lastExportTimestamp = now
		return true
	}

	return false
}
