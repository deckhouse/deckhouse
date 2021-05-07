package db

import (
	"fmt"

	dbcontext "d8.io/upmeter/pkg/db/context"
)

// SQLite: "No Isolation Between Operations On The Same Database Connection"
// https://www.sqlite.org/isolation.html

// Wrap database context with a transaction
func WithTx(ctx *dbcontext.DbContext, callback func(tx *dbcontext.DbContext) error) error {
	trans, err := NewTx(ctx)
	if err != nil {
		return err
	}
	err = callback(trans.Ctx())
	return trans.Act(err)
}

type Trans struct {
	parent *dbcontext.DbContext
	tx     *dbcontext.DbContext
}

// NewTx wraps the transaction. Use Ctx() for connection handling and don't forget to call .Act(error) in the end.
func NewTx(ctx *dbcontext.DbContext) (*Trans, error) {
	parent := ctx.Start()

	tx, err := parent.BeginTransaction()
	if err != nil {
		parent.Stop()
		return nil, err
	}

	t := &Trans{
		parent: parent,
		tx:     tx,
	}

	return t, nil
}

// Ctx returns the transaction DB context
func (t *Trans) Ctx() *dbcontext.DbContext {
	return t.tx.Start()
}

// Act commits the transaction if nil passed, and returns nil in this case. If an error passed or occurred
// on the commit, the  transaction is rolled back.
func (t *Trans) Act(err error) error {
	defer t.parent.Stop()

	if err != nil {
		return t.rollback(err)
	}

	err = t.tx.Commit()
	if err != nil {
		return t.rollback(err)
	}
	return nil
}

func (t *Trans) rollback(err error) error {
	rbErr := t.tx.Rollback()
	if rbErr != nil {
		return fmt.Errorf("cannot rollback on %v: %v", err, rbErr)
	}
	return err
}
