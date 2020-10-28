package util

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func Connect(path string, dbhInjector ...func(*sql.DB)) error {
	// busy_time and MaxOpenConns help eliminate errors "database is locked"
	// See https://github.com/mattn/go-sqlite3/issues/274
	// https://github.com/mattn/go-sqlite3#faq
	// Can I use this in multiple routines concurrently?
	// Yes for readonly. But, No for writable. See #50, #51, #209, #274.
	dbh, err := sql.Open("sqlite3", path+"?_busy_timeout=9999999")
	if err != nil {
		return fmt.Errorf("open db '%s': %v", path, err)
	}

	dbh.SetMaxOpenConns(1)

	if len(dbhInjector) > 0 && dbhInjector[0] != nil {
		dbhInjector[0](dbh)
	}

	return nil
}
