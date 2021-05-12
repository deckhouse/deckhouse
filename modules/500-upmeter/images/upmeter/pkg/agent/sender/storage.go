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
	List(*dbcontext.DbContext) ([]check.Episode, error)
	Clean(*dbcontext.DbContext, time.Time) error
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

func (s *ListStorage) List() ([]check.Episode, error) {
	trans, err := db.NewTx(s.ctx)
	if err != nil {
		return nil, err
	}
	episodes, err := s.inner.List(trans.Ctx())
	return episodes, trans.Act(err)
}

func (s *ListStorage) Clean(slot time.Time) error {
	return db.WithTx(s.ctx, func(tx *dbcontext.DbContext) error {
		return s.inner.Clean(tx, slot)
	})
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

func (w *wal) List(tx *dbcontext.DbContext) ([]check.Episode, error) {
	// The EpisodeDao30s object contains hardcoded table name
	db := dao.NewEpisodeDao30s(tx)
	slot, err := db.GetEarliestTimeSlot()
	if err != nil {
		return nil, err
	}
	return db.ListEpisodesBySlot(slot)
}
