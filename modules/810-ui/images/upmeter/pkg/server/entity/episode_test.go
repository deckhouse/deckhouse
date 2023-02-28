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

package entity

import (
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/server/ranges"
)

func Test_Save30sEpisode_Saves(t *testing.T) {
	dao := newMemoryEpisodeDao30s(t)

	want := check.Episode{
		ProbeRef: check.ProbeRef{Group: "ah", Probe: "oh"},
		TimeSlot: time.Now().Truncate(30 * time.Second),
		Up:       28 * time.Second,
		Down:     300 * time.Millisecond,
		Unknown:  600 * time.Millisecond,
		NoData:   1100 * time.Millisecond,
	}

	saved, err := save30sEpisode(dao.DbCtx, want)
	if err != nil {
		t.Errorf("cannot save 30s episode: %v", err)
	}

	got := *saved
	if !reflect.DeepEqual(want, got) {
		t.Errorf("was not returned what we have saved, want=%v, got=%v", want, got)
	}

	from := want.TimeSlot.Truncate(30 * time.Second)
	to := from.Add(30 * time.Second)

	items, err := dao.ListByRange(from, to, want.ProbeRef)
	if err != nil {
		t.Errorf("cannot list saved 30s episode: %v", err)
	}

	if len(items) == 0 {
		t.Errorf("no items saved for 30s episode")
		t.FailNow()
	}

	got = items[0].Episode
	if !reflect.DeepEqual(want, got) {
		t.Errorf("did not get what we have saved, want=%v, got=%v", want, got)
	}
}

func Test_Save30sEpisode_Combines(t *testing.T) {
	dao := newMemoryEpisodeDao30s(t)

	initial := check.Episode{
		ProbeRef: check.ProbeRef{Group: "ah", Probe: "oh"},
		TimeSlot: time.Now().Truncate(30 * time.Second),
		Up:       28 * time.Second,
		Down:     300 * time.Millisecond,
		Unknown:  600 * time.Millisecond,
		NoData:   1100 * time.Millisecond,
	}

	want := check.Episode{
		ProbeRef: initial.ProbeRef,
		TimeSlot: initial.TimeSlot,
		Up:       initial.Up + 200*time.Millisecond,
		Down:     initial.Down - 50*time.Millisecond,
		Unknown:  initial.Unknown - 150*time.Millisecond,
		NoData:   initial.NoData,
	}

	_, err := save30sEpisode(dao.DbCtx, initial)
	saved, err := save30sEpisode(dao.DbCtx, want)
	if err != nil {
		t.Errorf("cannot save 30s episode: %v", err)
	}

	got := *saved
	if !reflect.DeepEqual(want, got) {
		t.Errorf("was not returned what we have saved, want=%v, got=%v", want, got)
	}

	from := want.TimeSlot.Truncate(30 * time.Second)
	to := from.Add(30 * time.Second)
	items, err := dao.ListByRange(from, to, want.ProbeRef)
	if err != nil {
		t.Errorf("cannot list saved 30s episode: %v", err)
	}
	if len(items) == 0 {
		t.Errorf("no items saved for 30s episode")
		t.FailNow()
	}

	got = items[0].Episode
	if !reflect.DeepEqual(want, got) {
		t.Errorf("did not get what we have saved, want=%v, got=%v", want, got)
	}
}

func Test_Update5mEpisode_Saves(t *testing.T) {
	dao := newMemoryEpisodeDao5m()

	initial30s := check.Episode{
		ProbeRef: check.ProbeRef{Group: "ah", Probe: "oh"},
		TimeSlot: time.Now(),
		Up:       28 * time.Second,
		Down:     300 * time.Millisecond,
		Unknown:  600 * time.Millisecond,
		NoData:   1100 * time.Millisecond,
	}

	slot5m := initial30s.TimeSlot.Truncate(5 * time.Minute)
	want5m := check.Episode{
		ProbeRef: initial30s.ProbeRef,
		TimeSlot: slot5m,
		Up:       initial30s.Up,
		Down:     initial30s.Down,
		Unknown:  initial30s.Unknown,
		NoData:   5*time.Minute - initial30s.Up - initial30s.Down - initial30s.Unknown,
	}

	_, err := save30sEpisode(dao.DbCtx, initial30s)
	if err != nil {
		t.Errorf("cannot save 30s episode: %v", err)
	}

	saved, err := update5mEpisode(dao.DbCtx, slot5m, initial30s.ProbeRef)
	if err != nil {
		t.Errorf("cannot save 5m episode: %v", err)
	}

	got5m := *saved
	if !reflect.DeepEqual(want5m, got5m) {
		t.Errorf("returned was not what we have saved, want=%v, got=%v", want5m, got5m)
	}

	from := initial30s.TimeSlot.Truncate(5 * time.Minute)
	to := from.Add(5 * time.Minute)
	timerange := ranges.New5MinStepRange(from.Unix(), to.Unix(), 300)
	entities, err := dao.ListEpisodeSumsForRanges(timerange, initial30s.ProbeRef)
	if err != nil {
		t.Errorf("cannot get 5m episodes: %v", err)
	}
	if len(entities) == 0 {
		t.Errorf("empty list in 5m entities")
		t.FailNow()
	}

	got5m = entities[0]
	if !reflect.DeepEqual(want5m, got5m) {
		t.Errorf("did not get what we have saved, want=%v, got=%v", want5m, got5m)
	}
}

func newMemoryEpisodeDao30s(t *testing.T) *dao.EpisodeDao30s {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		t.Fatalf("cannot connect to database: %v", err)
	}

	createCtx := dbctx.Start()
	defer createCtx.Stop()

	const query = `
	CREATE TABLE IF NOT EXISTS "episodes_30s" (
		timeslot        INTEGER NOT NULL,
		nano_up         INTEGER NOT NULL,
		nano_down       INTEGER NOT NULL,
		group_name      TEXT NOT NULL,
		probe_name      TEXT NOT NULL,
		nano_unknown    INTEGER NOT NULL DEFAULT 0,
		nano_unmeasured INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX downtime30s_time_group_probe ON episodes_30s (timeslot, group_name, probe_name);
	`

	_, err = createCtx.StmtRunner().Exec(query)
	if err != nil {
		t.Fatalf("cannot create table: %v", err)
	}

	return dao.NewEpisodeDao30s(dbctx)
}

func newMemoryEpisodeDao5m() *dao.EpisodeDao5m {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		panic(err)
	}

	createCtx := dbctx.Start()
	defer createCtx.Stop()

	const query = `
	CREATE TABLE IF NOT EXISTS "episodes_30s" (
		timeslot        INTEGER NOT NULL,
		nano_up         INTEGER NOT NULL,
		nano_down       INTEGER NOT NULL,
		group_name      TEXT NOT NULL,
		probe_name      TEXT NOT NULL,
		nano_unknown    INTEGER NOT NULL DEFAULT 0,
		nano_unmeasured INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX downtime30s_time_group_probe ON episodes_30s (timeslot, group_name, probe_name);

	CREATE TABLE IF NOT EXISTS "episodes_5m" (
		timeslot        INTEGER NOT NULL,
		nano_up         INTEGER NOT NULL,
		nano_down       INTEGER NOT NULL,
		group_name      TEXT NOT NULL,
		probe_name      TEXT NOT NULL,
		nano_unknown    INTEGER NOT NULL DEFAULT 0,
		nano_unmeasured INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX downtime5m_time_group_probe ON "episodes_5m" (timeslot, group_name, probe_name);
	`

	_, err = createCtx.StmtRunner().Exec(query)
	if err != nil {
		panic(err)
	}

	return dao.NewEpisodeDao5m(dbctx)
}
