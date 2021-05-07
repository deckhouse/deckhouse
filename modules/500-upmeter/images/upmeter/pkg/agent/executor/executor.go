package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/agent/manager"
	"d8.io/upmeter/pkg/check"
)

const (
	exportPeriod = 30 * time.Second
	scrapePeriod = 200 * time.Millisecond
)

func seriesSize() int {
	return int(exportPeriod / scrapePeriod)
}

type ProbeExecutor struct {
	probeManager *manager.Manager
	metrics      *metric_storage.MetricStorage

	// to receive results from runners
	recv    chan check.Result
	series  map[string]*check.StatusSeries
	results map[string]*check.ProbeResult

	// to send a bunch of episodes further
	send chan []check.Episode
}

func New(mgr *manager.Manager, send chan []check.Episode) *ProbeExecutor {
	p := &ProbeExecutor{
		recv:    make(chan check.Result),
		series:  make(map[string]*check.StatusSeries),
		results: make(map[string]*check.ProbeResult),

		probeManager: mgr,
		send:         send,
	}
	return p
}

func (e *ProbeExecutor) Start(ctx context.Context) {
	go e.runTicker(ctx)
	go e.scrapeTicker(ctx)
}

// runTicker is the scheduler for probe checks
func (e *ProbeExecutor) runTicker(ctx context.Context) {
	ticker := time.NewTicker(scrapePeriod)

	for {
		select {
		case <-ticker.C:
			// TODO: pass context to checks
			e.run()
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// scrapeTicker collects probe check results and schedules the exporting of episodes.
func (e *ProbeExecutor) scrapeTicker(ctx context.Context) {
	// we want to be able to align with the real exportPeriod in time and to spawn before we run a probe
	ticker := time.NewTicker(scrapePeriod)

	for {
		select {
		case <-ticker.C:
			err := e.scrape()
			if err != nil {
				log.Fatalf("cannot scrape results: %v", err)
			}
		case result := <-e.recv:
			e.collect(result)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// run checks if probe is running and restarts them
func (e *ProbeExecutor) run() {
	// rounding lets us avoid inaccuracies in time comparison
	now := time.Now().Round(scrapePeriod)

	for _, runner := range e.probeManager.Runners() {
		if !runner.ShouldRun(now) {
			continue
		}

		runner := runner // avoid closure capturing
		go func() {
			e.recv <- runner.Run(now)

			e.metrics.CounterAdd(
				"upmeter_agent_probe_run_total",
				1.0,
				map[string]string{"probe": runner.ProbeRef().Id()},
			)
		}()
	}
}

// collect stores the check result in the intermediate format
func (e *ProbeExecutor) collect(checkResult check.Result) {
	id := checkResult.ProbeRef.Id()
	probeResult, ok := e.results[id]
	if !ok {
		probeResult = check.NewProbeResult(*checkResult.ProbeRef)
		e.results[id] = probeResult
	}
	probeResult.Add(checkResult)
}

// scrape checks probe results
func (e *ProbeExecutor) scrape() error {
	var (
		now        = time.Now()
		exportTime = now.Round(exportPeriod)
		scrapeTime = now.Round(scrapePeriod)
	)

	for id, probeResult := range e.results {
		series, ok := e.series[id]
		if !ok {
			series = check.NewStatusSeries(seriesSize())
			e.series[id] = series
		}
		err := series.Add(probeResult.Status())
		if err != nil {
			return fmt.Errorf("cannot add series for probe %q: %v", id, err)
		}
	}

	if exportTime != scrapeTime {
		return nil
	}

	episodeStart := exportTime.Add(-exportPeriod)
	err := e.export(episodeStart)
	if err != nil {
		return fmt.Errorf("cannot export episodes: %v", err)
	}

	return nil
}

// export copies scraped results and sends them to sender along as evaluates computed probes.
func (e *ProbeExecutor) export(start time.Time) error {
	var episodes []check.Episode

	// collect episodes for calculated probes
	for _, calc := range e.probeManager.Calculators() {
		series, err := calcSeries(e.series, calc.MergeIds())
		if err != nil {
			return fmt.Errorf("cannot calculate episode stats for %q: %v", calc.ProbeRef().Id(), err)
		}
		ep := check.NewEpisode(calc.ProbeRef(), start, scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
	}

	// collect episodes for real probes
	for id, probeResult := range e.results {
		series := e.series[id]
		ep := check.NewEpisode(probeResult.ProbeRef(), start, scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
		series.Clean()
	}

	e.send <- episodes

	return nil
}

func calcSeries(byId map[string]*check.StatusSeries, ids []string) (*check.StatusSeries, error) {
	acc := check.NewStatusSeries(seriesSize())

	for _, id := range ids {
		series, ok := byId[id]
		if !ok {
			return nil, fmt.Errorf("series for %q is not present", id)
		}

		err := acc.Merge(series)
		if err != nil {
			return nil, fmt.Errorf("cannot merge status series: %v", err)
		}
	}

	return acc, nil
}
