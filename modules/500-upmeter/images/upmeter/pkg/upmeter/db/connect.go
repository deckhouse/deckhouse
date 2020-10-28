package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"upmeter/pkg/upmeter/db/dao"
	"upmeter/pkg/upmeter/db/migrations"
	"upmeter/pkg/upmeter/db/util"
)

func Connect(path string, dbhInjector ...func(*sql.DB)) error {
	if len(dbhInjector) == 0 {
		dbhInjector = append(dbhInjector, DefaultDbhInjector)
	}

	return util.Connect(path, dbhInjector...)
}

// InjectDbh injects dbh into all default Dao
func DefaultDbhInjector(dbh *sql.DB) {
	dao.Downtime30s.Dbh = dbh
	dao.Downtime5m.Dbh = dbh
	migrations.Migrator.Dbh = dbh
}
