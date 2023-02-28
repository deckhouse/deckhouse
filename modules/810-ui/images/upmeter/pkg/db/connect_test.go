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
	"os"
	"sync"
	"testing"

	dbcontext "d8.io/upmeter/pkg/db/context"
)

// Reproduce messages "insert error: database is locked" without MaxOpenConns and busy_timeout in Connect.
// 1000 iterations in WriteWorker are not enough to reproduce.
// 10000 iterations in WriteWorker give one or more messages and there are missing records.
// sqlite> select count(1) from test where num=5123;
// 10000
// sqlite> select count(1) from test where num=431;
// 10000
// sqlite> select count(1) from test where num=12;
// 9999
func Test_reproduce_database_is_locked(t *testing.T) {
	t.SkipNow()

	mainDbCtx, err := Connect("test.sqlite", nil)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect error: %+v\n", err)
		t.Fail()
	}
	dbCtx := mainDbCtx.Start()
	defer mainDbCtx.Stop()

	_, _ = dbCtx.StmtRunner().Exec(CreateTableTest)

	workers := []Worker{
		&WriteWorker{Num: 12, DbCtx: dbCtx},
		&WriteWorker{Num: 431, DbCtx: dbCtx},
		&WriteWorker{Num: 5123, DbCtx: dbCtx},
		&ReadWorker{Num: 5123, DbCtx: dbCtx},
		&ReadWorker{Num: 12, DbCtx: dbCtx},
	}

	var wg sync.WaitGroup
	wg.Add(len(workers))
	for _, wrk := range workers {
		wrk.Start(&wg)
	}

	wg.Wait()
}

const CreateTableTest = `
CREATE TABLE IF NOT EXISTS test (
	num INTEGER NOT NULL
)
`

type Worker interface {
	Start(wg *sync.WaitGroup)
}

type WriteWorker struct {
	DbCtx *dbcontext.DbContext
	Num   int64
}

func (w *WriteWorker) Start(wg *sync.WaitGroup) {
	go func() {
		for i := 0; i < 10000; i++ {
			_, err := w.DbCtx.StmtRunner().Exec(`insert into test (num) values (?)`, w.Num)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "insert error: %+v\n", err)
			}
		}
		wg.Done()
	}()
}

type ReadWorker struct {
	DbCtx *dbcontext.DbContext
	Num   int64
}

func (w *ReadWorker) Start(wg *sync.WaitGroup) {
	go func() {
		for i := 0; i < 100; i++ {
		}
		wg.Done()
	}()
}
