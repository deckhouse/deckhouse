package db

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
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
	workers := []Worker{
		&WriteWorker{Num: 12},
		&WriteWorker{Num: 431},
		&WriteWorker{Num: 5123},
		//&ReadWorker{Num: 5123},
		//&ReadWorker{Num: 12},
	}

	var dbh *sql.DB

	err := Connect("test.sqlite", func(db *sql.DB) {
		dbh = db
		for _, wrk := range workers {
			wrk.WithDbh(db)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error: %+v\n", err)
		t.Fail()
	}

	dbh.Exec(CreateTableTest)

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
	WithDbh(dh *sql.DB)
	Start(wg *sync.WaitGroup)
}

type WriteWorker struct {
	Dbh *sql.DB
	Num int64
}

func (w *WriteWorker) WithDbh(db *sql.DB) {
	w.Dbh = db
}

func (w *WriteWorker) Start(wg *sync.WaitGroup) {
	go func() {
		for i := 0; i < 10000; i++ {
			_, err := w.Dbh.Exec(`insert into test (num) values (?)`, w.Num)
			if err != nil {
				fmt.Fprintf(os.Stderr, "insert error: %+v\n", err)
			}
		}
		wg.Done()
	}()
}

type ReadWorker struct {
	Dbh *sql.DB
	Num int64
}

func (w *ReadWorker) WithDbh(db *sql.DB) {
	w.Dbh = db
}

func (w *ReadWorker) Start(wg *sync.WaitGroup) {
	go func() {
		for i := 0; i < 100; i++ {
			rows, err := w.Dbh.Query(`select * from test where num = ?`, w.Num)
			if err != nil {
				fmt.Fprintf(os.Stderr, "select %d error: %+v\n", w.Num, err)
				continue
			}

			var res = make([]int64, 0)
			for rows.Next() {
				var ref int64 = 0
				err := rows.Scan(&ref)
				if err != nil {
					fmt.Fprintf(os.Stderr, "select %d rows error: %+v\n", w.Num, err)
					continue
				}
				res = append(res, ref)
			}
		}
		wg.Done()
	}()
}
