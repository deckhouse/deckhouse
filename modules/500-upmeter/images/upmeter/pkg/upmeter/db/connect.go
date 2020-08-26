package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func Connect(path string, dbhInjector ...func(*sql.DB)) error {
	dbh, err := sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("open db '%s': %v", path, err)
	}

	err = EnsureTables(dbh)
	if err != nil {
		return fmt.Errorf("ensure tables: %v", err)
	}

	if len(dbhInjector) > 0 && dbhInjector[0] != nil {
		dbhInjector[0](dbh)
	} else {
		InjectDbh(dbh)
	}

	return nil
}

// InjectDbh injects dbh into all default Dao
func InjectDbh(dbh *sql.DB) {
	Downtime30s.Dbh = dbh
	Downtime5m.Dbh = dbh
}
