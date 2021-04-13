package migrations

import (
	"testing"

	dbcontext "upmeter/pkg/upmeter/db/context"
)

func GetTestDatabase() *dbcontext.DbContext {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect("file::memory:")
	if err != nil {
		panic(err)
	}

	err = MigrateDatabase(dbctx, "./server")
	if err != nil {
		panic(err)
	}
	return dbctx
}

func Test_all_migrations(t *testing.T) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect("file::memory:")
	if err != nil {
		panic(err)
	}

	err = MigrateDatabase(dbctx, "./server")
	if err != nil {
		t.Errorf("error running migrations from ground up: %v", err)
	}
}

// this test tries to verify we now reproducible migrations independent on the state of the db in upmeter for the range
// of migrations from 1 to 4.
func Test_migrations_for_existing_schema(t *testing.T) {
	const (
		// This range (from 1 to 3) is fixed by the database state in deckhouse releases 21.01 and 21.02.
		// It is not expected to work for migrations starting from #4.
		min = 1
		max = 3
	)
	for currentStep := min; currentStep < max; currentStep++ {
		dbctx := dbcontext.NewDbContext()
		err := dbctx.Connect("file::memory:")
		if err != nil {
			panic(err)
		}

		err = migrateServer(dbctx, currentStep)
		if err != nil {
			t.Fatalf("error running migrations from ground up: %v", err)
		}

		err = deleteMigrationData(dbctx)
		if err != nil {
			t.Fatalf("error dropping migrations table: %v", err)
		}

		err = MigrateDatabase(dbctx, "./server")
		if err != nil {
			t.Fatalf("error running migrations for existing outdated schema %d: %v", currentStep, err)
		}

	}
}

// migrate twice to check we don't fail when everything is ok
func Test_uptodate_migrations_state(t *testing.T) {
	dbctx := dbcontext.NewDbContext()
	err := dbctx.Connect("file::memory:")
	if err != nil {
		panic(err)
	}

	err = MigrateDatabase(dbctx, "./server")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}

	err = MigrateDatabase(dbctx, "./server")
	if err != nil {
		t.Fatalf("error running migrations from ground up: %v", err)
	}
}

func deleteMigrationData(dbctx *dbcontext.DbContext) error {
	ctx := dbctx.Start()
	defer ctx.Stop()

	_, err := ctx.StmtRunner().Exec("DROP TABLE schema_migrations;")
	return err
}
