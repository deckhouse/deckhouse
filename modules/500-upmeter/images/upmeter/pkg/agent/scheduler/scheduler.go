/*
Copyright 2021 Flant JSC

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
	recv       chan check.Result
	seriesMap  map[string]*check.StatusSeries
	resultsMap map[string]*check.ProbeResult

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
		recv:       make(chan check.Result),
		seriesMap:  make(map[string]*check.StatusSeries),
		resultsMap: make(map[string]*check.ProbeResult),

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

	// Export time in future
	exportTime := time.Now().Truncate(e.exportPeriod).Add(e.exportPeriod)

	for {
		select {
		case result := <-e.recv:
			// Put check result to a probe result in common results map
			e.collect(result)

		case <-ticker.C:
			scrapeTime := time.Now().Truncate(e.scrapePeriod)

			// Is it time to export the data?
			if scrapeTime.Equal(exportTime) || scrapeTime.After(exportTime) {
				episodeStart := exportTime.Add(-e.exportPeriod)
				episodes, err := e.convert(episodeStart)
				if err != nil {
					log.Fatalf("exporting results: %v", err)
				}
				go func() {
					// Exporting to sender
					e.send <- episodes
				}()

				// Cleaning allocated series space
				for _, series := range e.seriesMap {
					series.Clean()
				}

				exportTime = exportTime.Add(e.exportPeriod)
			}

			// Add probe statuses to status series
			err := e.scrape(scrapeTime)
			if err != nil {
				log.Fatalf("cannot scrape results: %v", err)
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
	probeResult, ok := e.resultsMap[id]
	if !ok {
		probeResult = check.NewProbeResult(*checkResult.ProbeRef)
		e.resultsMap[id] = probeResult
	}
	probeResult.Add(checkResult)

	// Init series for the same ID
	if _, ok := e.seriesMap[id]; !ok {
		e.seriesMap[id] = check.NewStatusSeries(e.seriesSize)
	}
}

// scrape checks probe results, add them to their status series
func (e *Scheduler) scrape(scrapeTime time.Time) error {
	// Within the episode, how far we have reached
	exportTimeRemainder := scrapeTime.UnixNano() % int64(e.exportPeriod)
	// What point in the series are we in right now
	scrapeIndex := int(exportTimeRemainder / int64(e.scrapePeriod))

	for id, probeResult := range e.resultsMap {
		series := e.seriesMap[id]
		err := series.AddI(scrapeIndex, probeResult.Status())
		if err != nil {
			return fmt.Errorf("adding series for probe %q: %v", id, err)
		}
	}
	return nil
}

func (e *Scheduler) convert(start time.Time) ([]check.Episode, error) {
	episodes := make([]check.Episode, 0, len(e.resultsMap))

	// Collect episodes for calculated probes.
	for _, calc := range e.registry.Calculators() {
		sss := make([]*check.StatusSeries, 0)
		for _, id := range calc.MergeIds() {
			if ss, ok := e.seriesMap[id]; ok {
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
	for id, probeResult := range e.resultsMap {
		series := e.seriesMap[id]
		// Calculated probe series contain no new data, so they are skipped.
		group := probeResult.ProbeRef().Group
		if _, ok := byGroup[group]; !ok {
			byGroup[group] = make([]*check.StatusSeries, 0)
		}
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
