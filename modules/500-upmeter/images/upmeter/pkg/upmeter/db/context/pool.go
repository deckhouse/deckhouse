package context

import (
	"database/sql"
	"fmt"
	"net/url"

	"upmeter/pkg/util"
)

/**
 * tldr; A set of parameters for 'Open' and a custom pool implementation
 * to eliminate "database is locked" errors.
 *
 * Problem description and some info is here: https://github.com/mattn/go-sqlite3/issues/274
 *
 * First, we use busy_timeout+MaxOpenConns(1) as a common workaround.
 * Next, custom pool helps to stick queries and transactions to connections.
 *   (See https://turriate.com/articles/making-sqlite-faster-in-go)
 * And finally, set _txlock=immediate to start transactions in write mode.
 *   (See https://www.sqlite.org/lang_transaction.html)
 */

var defaultOpenFlags = "_busy_timeout=9999999&_txlock=immediate"
var MaxOpenConnections = 1
var PoolSize = 20

// Open opens a sqlite database
func Open(path string, params map[string]string) (*sql.DB, error) {
	// busy_time and MaxOpenConns help eliminate errors "database is locked"
	// See https://github.com/mattn/go-sqlite3/issues/274
	// https://github.com/mattn/go-sqlite3#faq
	// Can I use this in multiple routines concurrently?
	// Yes for readonly. But, No for writable. See #50, #51, #209, #274.

	openFlags, err := url.ParseQuery(defaultOpenFlags)
	if err != nil {
		return nil, err
	}
	for k, v := range params {
		openFlags.Set(k, v)
	}

	dbh, err := sql.Open("sqlite3", path+"?"+openFlags.Encode())
	if err != nil {
		return nil, fmt.Errorf("open db '%s': %v", path, err)
	}

	dbh.SetMaxOpenConns(MaxOpenConnections)

	return dbh, nil
}

// ConnPool is a custom database connections pool.
type ConnPool struct {
	PoolSize int
	Conns    chan *sql.DB
}

func NewConnPool() *ConnPool {
	poolSize := util.GetenvInt64("UPMETER_DB_POOL_SIZE")
	if poolSize == 0 {
		poolSize = PoolSize
	}
	return &ConnPool{
		PoolSize: poolSize,
		Conns:    make(chan *sql.DB, poolSize),
	}
}

func (c *ConnPool) Capture() *sql.DB {
	return <-c.Conns
}

func (c *ConnPool) Release(conn *sql.DB) {
	c.Conns <- conn
}

func (c *ConnPool) Connect(path string, poolInjector ...func(pool *ConnPool)) error {
	return c.ConnectWithParams(path, nil, poolInjector...)
}

func (c *ConnPool) ConnectWithParams(path string, params map[string]string, poolInjector ...func(pool *ConnPool)) error {
	for i := 0; i < c.PoolSize; i++ {
		var dbh *sql.DB
		dbh, err := Open(path, params)
		if err != nil {
			return err
		}
		if len(poolInjector) > 0 && poolInjector[0] != nil {
			poolInjector[0](c)
		}
		c.Conns <- dbh
	}

	return nil
}
