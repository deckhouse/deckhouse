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

package sender

import (
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
)

type ListTxStorage interface {
	Save(*dbcontext.DbContext, []check.Episode) error
	List(*dbcontext.DbContext, int) ([]check.Episode, error)
	Clean(*dbcontext.DbContext, time.Time) error
	CountSlots(*dbcontext.DbContext) (int, error)
}

// ListStorage manages the transaction for ListTxStorage
type ListStorage struct {
	inner ListTxStorage
	ctx   *dbcontext.DbContext
}

func NewListStorage(inner ListTxStorage, ctx *dbcontext.DbContext) *ListStorage {
	return &ListStorage{
		inner: inner,
		ctx:   ctx,
	}
}

func (s *ListStorage) Save(episodes []check.Episode) error {
	return db.WithTx(s.ctx, func(tx *dbcontext.DbContext) error {
		return s.inner.Save(tx, episodes)
	})
}

// List returns episodes of up to maxSlots earliest time slots in a single batch.
func (s *ListStorage) List(maxSlots int) ([]check.Episode, error) {
	trans, err := db.NewTx(s.ctx)
	if err != nil {
		return nil, err
	}
	episodes, err := s.inner.List(trans.Ctx(), maxSlots)
	return episodes, trans.Act(err)
}

func (s *ListStorage) Clean(slot time.Time) error {
	return db.WithTx(s.ctx, func(tx *dbcontext.DbContext) error {
		return s.inner.Clean(tx, slot)
	})
}

// CountSlots returns how many distinct time slots are pending in the WAL (the backlog size).
func (s *ListStorage) CountSlots() (int, error) {
	trans, err := db.NewTx(s.ctx)
	if err != nil {
		return 0, err
	}
	count, err := s.inner.CountSlots(trans.Ctx())
	return count, trans.Act(err)
}

func NewStorage(dbctx *dbcontext.DbContext) *ListStorage {
	return NewListStorage(&wal{}, dbctx)
}

type wal struct{}

func (w *wal) Save(tx *dbcontext.DbContext, episodes []check.Episode) error {
	// The EpisodeDao30s object contains hardcoded table name
	db := dao.NewEpisodeDao30s(tx)
	return db.SaveBatch(episodes)
}

func (w *wal) Clean(tx *dbcontext.DbContext, slot time.Time) error {
	// The EpisodeDao30s object contains hardcoded table name
	db := dao.NewEpisodeDao30s(tx)
	return db.DeleteUpTo(slot)
}

func (w *wal) List(tx *dbcontext.DbContext, maxSlots int) ([]check.Episode, error) {
	// The EpisodeDao30s object contains hardcoded table name
	db := dao.NewEpisodeDao30s(tx)
	return db.ListEpisodesUpToNSlots(maxSlots)
}

func (w *wal) CountSlots(tx *dbcontext.DbContext) (int, error) {
	// The EpisodeDao30s object contains hardcoded table name
	db := dao.NewEpisodeDao30s(tx)
	return db.CountDistinctTimeSlots()
}
