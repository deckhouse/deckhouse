package migrations

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	dbcontext "upmeter/pkg/upmeter/db/context"
)

func MigrateDatabase(dbctx *dbcontext.DbContext, migrationsPath string) error {
	m, err := newMigrate(dbctx, migrationsPath)
	if err != nil {
		return err
	}
	return forbidNoChange(m.Up())
}

func migrateServer(dbctx *dbcontext.DbContext, steps int) error {
	m, err := newMigrate(dbctx, "./server")
	if err != nil {
		return err
	}
	return forbidNoChange(m.Steps(steps))
}

func forbidNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func newMigrate(dbctx *dbcontext.DbContext, migrationsPath string) (*migrate.Migrate, error) {
	ctx := dbctx.Start()
	defer ctx.Stop()

	driver, err := sqlite3.WithInstance(ctx.Dbh, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("cannot init driver: %v", err)
	}

	sourceURL := "file://" + migrationsPath
	return migrate.NewWithDatabaseInstance(sourceURL, "migration", driver)
}
