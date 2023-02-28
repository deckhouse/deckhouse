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

package context

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultPoolSize = 20
)

type DbContext struct {
	conns *pool
	db    *sql.DB
	tx    *sql.Tx
}

func NewDbContext() *DbContext {
	return &DbContext{}
}

// Connect opens a DB without pooling. Mainly for tests.
func (c *DbContext) Connect(path string) error {
	db, err := open(path, nil)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func parseIntEnvVar(name string) int {
	s := os.Getenv(name)
	if s == "" || s == "0" {
		return 0
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// ConnectWithPool creates a pool of DB connections.
func (c *DbContext) ConnectWithPool(path string, opts map[string]string) error {
	size := parseIntEnvVar("UPMETER_DB_POOL_SIZE") // FIXME bring out to the app arguments
	if size == 0 {
		size = defaultPoolSize
	}
	c.conns = newPool(size)

	return c.conns.Connect(path, opts)
}

func (c *DbContext) Handler() *sql.DB {
	return c.db
}

func (c *DbContext) Copy() *DbContext {
	return &DbContext{
		conns: c.conns,
		db:    c.db,
		tx:    c.tx,
	}
}

func (c *DbContext) StmtRunner() StmtRunner {
	if c.tx != nil {
		return c.tx
	}

	if c.db != nil {
		return c.db
	}

	panic("DB context is uninitialized")
}

// Start captures a connection from pool and returns a stoppable context.
// If context is stoppable, returns non-stoppable db-only context.
func (c *DbContext) Start() *DbContext {
	if c.tx != nil {
		return &DbContext{tx: c.tx}
	}

	// Do not copy pool if the db is already captured.
	if c.db != nil {
		return &DbContext{db: c.db}
	}

	// Capture connection from the pool if it is a "root" context.
	if c.conns != nil && c.db == nil {
		db := c.conns.Capture()
		return &DbContext{
			conns: c.conns,
			db:    db,
		}
	}

	panic("Call Start from uninitialized DbContext")
}

func (c *DbContext) Stop() {
	if c.conns != nil && c.db != nil {
		c.conns.Release(c.db)
	}
}

// BeginTransaction starts a transaction with default driver options: the isolation level and the readonly flag.
func (c *DbContext) BeginTransaction() (*DbContext, error) {
	ctx := context.Background() // FIXME (e.shevchenko) pass the context from the outside

	if c.tx != nil {
		return &DbContext{tx: c.tx}, nil
	}

	if c.db == nil {
		return nil, fmt.Errorf("begin transaction from uninitialized DbContext")
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &DbContext{tx: tx}, nil
}

func (c *DbContext) Rollback() error {
	if c.tx != nil {
		return c.tx.Rollback()
	}
	return nil
}

func (c *DbContext) Commit() error {
	if c.tx != nil {
		return c.tx.Commit()
	}
	return nil
}
