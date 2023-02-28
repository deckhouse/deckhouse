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
	"testing"

	dbcontext "d8.io/upmeter/pkg/db/context"
)

func Test_server_migrations(t *testing.T) {
	GetTestMemoryDatabase(t, "./server")
}

func Test_agent_migrations(t *testing.T) {
	GetTestMemoryDatabase(t, "./agent")
}

// migrate twice to check we don't fail when everything is ok
func Test_repeated_server_migrations(t *testing.T) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		t.Fatalf("cannot connect to database: %v", err)
	}

	err = MigrateDatabase(context.TODO(), dbctx, "./server")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}

	err = MigrateDatabase(context.TODO(), dbctx, "./server")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}
}

// migrate twice to check we don't fail when everything is ok
func Test_repeated_agent_migrations(t *testing.T) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		t.Fatalf("cannot connect to database: %v", err)
	}

	err = MigrateDatabase(context.TODO(), dbctx, "./agent")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}

	err = MigrateDatabase(context.TODO(), dbctx, "./agent")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}
}

// this test tries to verify we now reproducible migrations independent on the state of the db in upmeter for the range
// of migrations from 1 to 4.
func Test_migrations_for_existing_schema(t *testing.T) {
	t.SkipNow()

	const (
		// This range (from 1 to 3) is fixed by the database state in deckhouse releases 21.01 and 21.02.
		// It is not expected to work for migrations starting from #4.
		min = 1
		max = 3
	)

	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}

	for i := min; i < max; i++ {
		err = migrateServer(dbctx, i)
		if err != nil {
			t.Fatalf("error running migrations from ground up (step %d/%d): %v", i, max, err)
		}

		err = deleteMigrationData(dbctx)
		if err != nil {
			t.Fatalf("error dropping migrations table (step %d/%d): %v", i, max, err)
		}

		err = MigrateDatabase(context.TODO(), dbctx, "./server")
		if err != nil {
			t.Fatalf("error running migrations for existing outdated schema %d: %v", i, err)
		}

	}
}

func Test_migrations_down_server(t *testing.T) {
	migrateUpAndDown(t, "./server")
}

func Test_migrations_down_agent(t *testing.T) {
	migrateUpAndDown(t, "./agent")
}

func migrateUpAndDown(t *testing.T, migrationsPath string) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect(":memory:")
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}

	m, err := newMigrate(dbctx, migrationsPath)
	if err != nil {
		t.Fatalf("cannot instantiate migration: %v", err)
	}
	if err = m.Up(); err != nil {
		t.Fatalf("cannot migrate up")
	}
	if err = m.Down(); err != nil {
		t.Fatalf("cannot migrate down")
	}
}

// migrateServer ensures server database is migrated to the desired step
func migrateServer(dbctx *dbcontext.DbContext, steps int) error {
	m, err := newMigrate(dbctx, "./server")
	if err != nil {
		return err
	}
	return forbidNoChange(m.Steps(steps))
}

func deleteMigrationData(dbctx *dbcontext.DbContext) error {
	ctx := dbctx.Start()
	defer ctx.Stop()

	_, err := ctx.StmtRunner().Exec("DROP TABLE schema_migrations;")
	return err
}
