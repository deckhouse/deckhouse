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

package scheduler

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/registry"
)

type Scheduler struct {
	registry *registry.Registry

	// to receive results from runners
	recv    chan check.Result
	series  map[string]*check.StatusSeries
	results map[string]*check.ProbeResult

	// time configuration
	exportPeriod time.Duration
	scrapePeriod time.Duration
	seriesSize   int

	// to send a bunch of episodes further
	send chan []check.Episode

	stop chan struct{}
	done chan struct{}
}

func New(reg *registry.Registry, send chan []check.Episode) *Scheduler {
	const (
		exportPeriod = 30 * time.Second
		scrapePeriod = 200 * time.Millisecond // minimal probe interval
	)

	return &Scheduler{
		recv:    make(chan check.Result),
		series:  make(map[string]*check.StatusSeries),
		results: make(map[string]*check.ProbeResult),

		exportPeriod: exportPeriod,
		scrapePeriod: scrapePeriod,
		seriesSize:   int(exportPeriod / scrapePeriod),

		registry: reg,
		send:     send,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (e *Scheduler) Start() {
	go e.runTicker()
	go e.scrapeTicker()
}

// runTicker is the scheduler for probe checks
func (e *Scheduler) runTicker() {
	ticker := time.NewTicker(e.scrapePeriod)

	for {
		select {
		case <-ticker.C:
			e.run()
		case <-e.stop:
			ticker.Stop()
			e.done <- struct{}{}
			return
		}
	}
}

// scrapeTicker collects probe check results and schedules the exporting of episodes.
func (e *Scheduler) scrapeTicker() {
	ticker := time.NewTicker(e.scrapePeriod)

	for {
		select {
		case result := <-e.recv:
			e.collect(result)

		case <-ticker.C:
			var (
				now        = time.Now()
				exportTime = now.Round(e.exportPeriod)
				scrapeTime = now.Round(e.scrapePeriod)
			)

			err := e.scrape()
			if err != nil {
				log.Fatalf("cannot scrape results: %v", err)
			}

			if exportTime != scrapeTime {
				continue
			}

			episodeStart := exportTime.Add(-e.exportPeriod)
			if err := e.export(episodeStart); err != nil {
				log.Fatalf("cannot export results: %v", err)
			}

		case <-e.stop:
			ticker.Stop()
			e.done <- struct{}{}
			return
		}
	}
}

// run checks if probe is running and restarts them
func (e *Scheduler) run() {
	// rounding lets us avoid inaccuracies in time comparison
	now := time.Now().Round(e.scrapePeriod)

	for _, runner := range e.registry.Runners() {
		if !runner.ShouldRun(now) {
			continue
		}

		runner := runner // avoid closure capturing
		go func() {
			e.recv <- runner.Run(now)
		}()
	}
}

// collect stores the check result in the intermediate format
func (e *Scheduler) collect(checkResult check.Result) {
	id := checkResult.ProbeRef.Id()
	probeResult, ok := e.results[id]
	if !ok {
		probeResult = check.NewProbeResult(*checkResult.ProbeRef)
		e.results[id] = probeResult
	}
	probeResult.Add(checkResult)
}

// scrape checks probe results
func (e *Scheduler) scrape() error {
	for id, probeResult := range e.results {
		series, ok := e.series[id]
		if !ok {
			series = check.NewStatusSeries(e.seriesSize)
			e.series[id] = series
		}
		err := series.Add(probeResult.Status())
		if err != nil {
			return fmt.Errorf("cannot add series for probe %q: %v", id, err)
		}
	}
	return nil
}

// export copies scraped results and sends them to sender along as evaluates computed probes.
func (e *Scheduler) export(start time.Time) error {
	episodes, err := e.convert(start)
	if err != nil {
		return err
	}

	// clean allocated series space
	for id := range e.results {
		series := e.series[id]
		series.Clean()
	}

	e.send <- episodes

	return nil
}

func (e *Scheduler) convert(start time.Time) ([]check.Episode, error) {
	episodes := make([]check.Episode, 0, len(e.results))

	// Collect episodes for calculated probes.
	for _, calc := range e.registry.Calculators() {
		sss := make([]*check.StatusSeries, 0)
		for _, id := range calc.MergeIds() {
			if ss, ok := e.series[id]; ok {
				sss = append(sss, ss)
			}
		}

		series, err := check.MergeStatusSeries(e.seriesSize, sss)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate episode stats for %q: %v", calc.ProbeRef().Id(), err)
		}

		ep := check.NewEpisode(calc.ProbeRef(), start, e.scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
	}

	// Collect episodes for real probes and sort series by group.
	byGroup := make(map[string][]*check.StatusSeries)
	for id, probeResult := range e.results {
		// Calculated probe series contain no new data, so they are skipped.
		group := probeResult.ProbeRef().Group
		if _, ok := byGroup[group]; !ok {
			byGroup[group] = make([]*check.StatusSeries, 0)
		}
		series := e.series[id]
		byGroup[group] = append(byGroup[group], series)

		ep := check.NewEpisode(probeResult.ProbeRef(), start, e.scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
	}

	// Collect group episodes.
	for group, probeSeriesList := range byGroup {
		groupSeries, err := check.MergeStatusSeries(e.seriesSize, probeSeriesList)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate episode stats for group %q: %v", group, err)
		}

		groupRef := check.ProbeRef{Group: group, Probe: dao.GroupAggregation}
		ep := check.NewEpisode(groupRef, start, e.scrapePeriod, groupSeries.Stats())
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

func (e *Scheduler) Stop() {
	close(e.stop)

	<-e.done
	<-e.done
}
