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

package migrations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
)

func GetMigratedDatabase(ctx context.Context, dbPath, migrationsPath string) (*dbcontext.DbContext, error) {
	// Setup db context with connection pool.
	dbctx, err := db.Connect(dbPath, dbcontext.DefaultConnectionOptions())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %v", err)
	}

	// Apply migrations
	err = MigrateDatabase(ctx, dbctx, migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate database: %v", err)
	}

	return dbctx, nil
}

func MigrateDatabase(ctx context.Context, dbctx *dbcontext.DbContext, migrationsPath string) error {
	m, err := newMigrate(dbctx, migrationsPath)
	if err != nil {
		return fmt.Errorf("cannot instantiate migration: %v", err)
	}

	migrationCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-migrationCtx.Done()
		m.GracefulStop <- true
	}()

	err = forbidNoChange(m.Up())
	if err == nil {
		err = ctx.Err()
	}

	return err
}

func forbidNoChange(err error) error {
	if err == nil || errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return fmt.Errorf("cannot migrate: %v", err)
}

func newMigrate(dbctx *dbcontext.DbContext, migrationsPath string) (*migrate.Migrate, error) {
	ctx := dbctx.Start()
	defer ctx.Stop()

	// no tx to be able to do vacuum
	driver, err := sqlite3.WithInstance(ctx.Handler(), &sqlite3.Config{NoTxWrap: true})
	if err != nil {
		return nil, fmt.Errorf("cannot init driver: %v", err)
	}

	sourceURL := "file://" + migrationsPath
	return migrate.NewWithDatabaseInstance(sourceURL, "migration", driver)
}

// Utility functions

func GetTestMemoryDatabase(t *testing.T, migrationPath string) *dbcontext.DbContext {
	dbctx, err := getTestDatabase(":memory:", migrationPath)
	if err != nil {
		t.Errorf("error running migrations from ground up: %v", err)
	}
	if dbctx == nil {
		t.Errorf("unexpected nil database context: %v", err)
	}
	return dbctx
}

func GetTestFileDatabase(t *testing.T, dbPath, migrationPath string) *dbcontext.DbContext {
	dbctx, err := getTestDatabase(dbPath, migrationPath)
	if err != nil {
		t.Errorf("error running migrations from ground up: %v", err)
	}
	if dbctx == nil {
		t.Errorf("unexpected nil database context: %v", err)
	}
	return dbctx
}

func getTestDatabase(dbPath, migrationPath string) (*dbcontext.DbContext, error) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(dbPath) // no connection pool
	if err != nil {
		return nil, err
	}

	err = MigrateDatabase(context.TODO(), dbctx, migrationPath)
	if err != nil {
		return nil, err
	}
	return dbctx, nil
}
