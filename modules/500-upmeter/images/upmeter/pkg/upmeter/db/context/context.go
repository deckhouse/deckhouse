package context

import (
	"context"
	"database/sql"
	"fmt"
)

type DbContext struct {
	ConnPool *ConnPool
	Dbh      *sql.DB
	Tx       *sql.Tx
}

func NewDbContext() *DbContext {
	return &DbContext{}
}

// Connect opens a DB without pooling. Mainly for tests.
func (c *DbContext) Connect(path string) error {
	dbh, err := OpenDB(path)
	if err != nil {
		return err
	}
	c.Dbh = dbh
	return nil
}

// ConnectWithPool creates a pool of DB connections.
func (c *DbContext) ConnectWithPool(path string) error {
	c.ConnPool = NewConnPool()
	return c.ConnPool.Connect(path)
}

func (c *DbContext) Copy() *DbContext {
	return &DbContext{
		ConnPool: c.ConnPool,
		Dbh:      c.Dbh,
		Tx:       c.Tx,
	}
}

func (c *DbContext) StmtRunner() StmtRunner {
	if c.Tx != nil {
		return c.Tx
	}

	if c.Dbh != nil {
		return c.Dbh
	}

	panic("Call StmtRunner from uninitialized DbContext")
}

// Start captures a connection from pool and returns a stoppable context.
// If context is stoppable, returns non-stoppable Dbh-only context.
func (c *DbContext) Start() *DbContext {
	if c.Tx != nil {
		return &DbContext{Tx: c.Tx}
	}

	// Do not copy ConnPool if Dbh is already captured.
	if c.Dbh != nil {
		return &DbContext{Dbh: c.Dbh}
	}

	// Capture connection from the pool if it is a "root" context.
	if c.ConnPool != nil && c.Dbh == nil {
		dbh := c.ConnPool.Capture()
		return &DbContext{
			ConnPool: c.ConnPool,
			Dbh:      dbh,
		}
	}

	panic("Call Start from uninitialized DbContext")
	return nil
}

func (c *DbContext) Stop() {
	if c.ConnPool != nil && c.Dbh != nil {
		c.ConnPool.Release(c.Dbh)
	}
}

func (c *DbContext) BeginTransaction() (*DbContext, error) {
	if c.Tx != nil {
		return &DbContext{Tx: c.Tx}, nil
	}

	if c.Dbh == nil {
		return nil, fmt.Errorf("begin transaction from uninitialized DbContext")
	}

	tx, err := c.Dbh.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	return &DbContext{Tx: tx}, nil
}

func (c *DbContext) Rollback() error {
	if c.Tx != nil {
		return c.Tx.Rollback()
	}

	return nil
}

func (c *DbContext) Commit() error {
	if c.Tx != nil {
		return c.Tx.Commit()
	}

	return nil
}
