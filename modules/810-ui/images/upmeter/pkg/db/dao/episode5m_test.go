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

package dao

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/migrations"
)

func Test_Fill_RandomDB_For_Today(t *testing.T) {
	// Unskip to generate test data for webui.
	t.SkipNow()

	dbCtx := getFileDatabase(t, "generated_random.db")
	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	dao30s := NewEpisodeDao30s(daoCtx)
	dao5m := NewEpisodeDao5m(daoCtx)

	groupProbes := map[string][]string{
		"control-plane": {
			"apiserver",
			"basic",
			"controller-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	epoch := time.Now().Add(-24 * time.Hour).Truncate(5 * time.Minute)
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			log.Infof("gen episodes for %s/%s", groupName, probeName)

			// 30 sec
			tsCount := 24 * 60 * 2
			for i := 0; i < tsCount; i++ {
				downtime := check.Episode{
					ProbeRef: check.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot: epoch.Add(time.Duration(i) * 30 * time.Second),
					Down:     time.Second * time.Duration(i%30-i%7-i%3),
					Up:       time.Second * time.Duration(30-i%30-i%7-i%3),
					Unknown:  time.Second * time.Duration(i%7),
					NoData:   time.Second * time.Duration(i%3),
				}
				dao30s.Insert(downtime)
			}

			// 5min
			step5m := 5 * 60
			tsCount = 24 * 12 // 12 episodes per hour
			for i := 0; i < tsCount; i++ {
				downtime := check.Episode{
					ProbeRef: check.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot: epoch.Add(time.Duration(i) * 5 * time.Minute), // firstTs + int64(step5m*i),
					Down:     time.Second * time.Duration(i%step5m),
					Up:       time.Second * time.Duration(step5m-i%step5m),
				}
				dao5m.Insert(downtime)
			}

		}
	}
}

// Test_Fill_30s_OneDay fills a database with random data to measure a size for 1 day.
func Test_Fill_30s_OneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := getFileDatabase(t, "generated_day_30s.db")

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	dao30s := NewEpisodeDao30s(daoCtx)

	groupProbes := map[string][]string{
		"control-plane": {
			"apiserver",
			"basic",
			"controller-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	epoch := time.Now().Add(-24 * time.Hour)
	tsCount := 24 * 60 * 2
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := check.Episode{
					ProbeRef: check.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot: epoch.Add(time.Duration(i) * 30 * time.Second),
					Down:     time.Second * time.Duration(i%30-i%4-i%5),
					Up:       time.Second * time.Duration(30-i%30-i%4-i%5),
					Unknown:  time.Second * time.Duration(i%4),
					NoData:   time.Second * time.Duration(i%5),
				}
				err = dao30s.Insert(downtime)
				g.Expect(err).ShouldNot(HaveOccurred())
			}
		}
	}
}

func Test_FillOneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := getFileDatabase(t, "generated_day_5m.db")

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	err = migrations.MigrateDatabase(context.TODO(), dbCtx, "../migrations/server")
	g.Expect(err).ShouldNot(HaveOccurred())

	dao5m := NewEpisodeDao5m(daoCtx)

	groupProbes := map[string][]string{
		"control-plane": {
			"apiserver",
			"basic",
			"controller-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	epoch := time.Now().Add(-24 * time.Hour)
	tsCount := 24 * 12 // 12 episodes per hour
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := check.Episode{
					ProbeRef: check.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot: epoch.Add(time.Duration(i) * 5 * time.Minute),
					Down:     time.Second * time.Duration(i%300-i%37-i%13),
					Up:       time.Second * time.Duration(300-i%300-i%37-i%13),
					Unknown:  time.Second * time.Duration(i%37),
					NoData:   time.Second * time.Duration(i%13),
				}
				dao5m.Insert(downtime)
			}
		}
	}
}

func Test_Fill_Year(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := getFileDatabase(t, "generated_year_5m.db")

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	err = migrations.MigrateDatabase(context.TODO(), dbCtx, "../migrations/server")
	g.Expect(err).ShouldNot(HaveOccurred())

	dao5m := NewEpisodeDao5m(daoCtx)

	groupProbes := map[string][]string{
		"control-plane": {
			"apiserver",
			"basic",
			"controller-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	epoch := time.Now().Add(-365 * 24 * time.Hour)
	tsCount := 365 * 24 * 12 // 12 episodes per hour
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := check.Episode{
					ProbeRef: check.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot: epoch.Add(time.Duration(i) * 5 * time.Minute),
					Down:     time.Second * time.Duration(i%300-i%31-i%17),
					Up:       time.Second * time.Duration(300-i%300-i%31-i%17),
					Unknown:  time.Second * time.Duration(i%17),
					NoData:   time.Second * time.Duration(i%31),
				}
				dao5m.Insert(downtime)
			}
		}
	}
}
