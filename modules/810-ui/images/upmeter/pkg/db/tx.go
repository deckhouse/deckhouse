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
