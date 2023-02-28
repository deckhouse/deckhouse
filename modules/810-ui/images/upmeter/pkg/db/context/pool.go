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
	"database/sql"
	"fmt"
	"net/url"
)

const (
	MaxOpenConnections = 1
)

// A set of parameters for 'Open' and a custom pool implementation to eliminate "database is locked" errors.
//
// Problem description and discussion: https://github.com/mattn/go-sqlite3/issues/274
//
// First, we use busy_timeout+MaxOpenConns(1) as a common workaround.
//
// Next, custom pool helps to stick queries and transactions to connections.
//
//	(See https://turriate.com/articles/making-sqlite-faster-in-go)
//
// And finally, set _txlock=immediate to start transactions in write mode.
//
//	(See https://www.sqlite.org/lang_transaction.html)
//
// busy_time and MaxOpenConns help eliminate errors "database is locked"
//
//	See     https://github.com/mattn/go-sqlite3/issues/274
//	        https://github.com/mattn/go-sqlite3#faq
//
// Can I use this in multiple routines concurrently?
//
//	Yes for readonly. But, No for writable. See #50, #51, #209, #274.
func DefaultConnectionOptions() map[string]string {
	return map[string]string{
		"_busy_timeout": "9999999",
		"_txlock":       "immediate",
	}
}

// open opens a sqlite database
func open(dbpath string, opts map[string]string) (*sql.DB, error) {
	uri := buildUri(dbpath, opts)

	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		return nil, fmt.Errorf("cannot open sqlite database %q: %v", uri, err)
	}

	db.SetMaxOpenConns(MaxOpenConnections)

	return db, nil
}

func buildUri(dbpath string, opts map[string]string) string {
	options := url.Values{}
	for k, v := range opts {
		options.Set(k, v)
	}

	if opts != nil {
		return dbpath + "?" + options.Encode()
	}
	return dbpath
}

// pool is a custom database connections pool.
type pool struct {
	size  int
	conns chan *sql.DB
}

func newPool(size int) *pool {
	return &pool{
		size:  size,
		conns: make(chan *sql.DB, size),
	}
}

func (c *pool) Capture() *sql.DB {
	return <-c.conns
}

func (c *pool) Release(conn *sql.DB) {
	c.conns <- conn
}

func (c *pool) Connect(path string, opts map[string]string, poolInjector ...func(pool *pool)) error {
	for i := 0; i < c.size; i++ {
		var db *sql.DB
		db, err := open(path, opts)
		if err != nil {
			return err
		}
		if len(poolInjector) > 0 && poolInjector[0] != nil {
			poolInjector[0](c)
		}
		c.conns <- db
	}

	return nil
}
