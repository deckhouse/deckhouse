package db

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

func EnsureTables(dbh *sql.DB) error {
	if os.Getenv("UPMETER_ENSURE_TABLES") == "no" {
		return nil
	}

	var shouldDrop bool
	if os.Getenv("UPMETER_DROP_TABLES") == "yes" {
		shouldDrop = true
	}

	ensures := []func() error{
		WrapEnsureTable(dbh, CreateTableDowntime30s, "downtime30s", shouldDrop),
		WrapEnsureTable(dbh, CreateTableDowntime5m, "downtime5m", shouldDrop),
	}

	for _, ensure := range ensures {
		if err := ensure(); err != nil {
			return err
		}
	}

	return nil
}

func WrapEnsureTable(dbh *sql.DB, createDdl string, tableName string, shouldDrop bool) func() error {
	return func() error {
		return EnsureTable(dbh, createDdl, tableName, shouldDrop)
	}
}

func EnsureTable(dbh *sql.DB, createDdl string, tableName string, shouldDrop bool) error {
	var err error

	if shouldDrop {
		dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
		_, err = dbh.Exec(dropQuery)
		if err != nil {
			return fmt.Errorf("drop '%s': %v", tableName, err)
		}
		log.Infof("table '%s' dropped", tableName)
	}

	_, err = dbh.Exec(createDdl)
	if err != nil {
		return fmt.Errorf("create '%s': %v", tableName, err)
	}
	log.Infof("table '%s' created", tableName)

	return nil
}
