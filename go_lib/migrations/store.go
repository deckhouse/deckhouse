package migrations

import (
	"errors"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var (
	store       *Store
	cmName      = "deckhouse-migrations"
	cmNamespace = "d8-system"
)

func init() {
	store = newStore()
}

type Store struct {
	initialized      bool
	deckhouseRelease *semver.Version
	migrations       []migration
}

type migration interface {
	Name() string
	Up(client k8s.Client) error
	Down(client k8s.Client) error
}

func newStore() *Store {
	return &Store{
		migrations: make([]migration, 0),
	}
}

func (s *Store) addMigration(migration migration) {
	s.migrations = append(s.migrations, migration)
}

/*
ConfigMap.

data:
  migrate_pdb: "1.45"
  create_another_parameter: "1.46"
  change_env_variables_for_grafana_deployment: "1.51"



data:
  001_migrate_pdb: "1.45"
  002_create_another_parameter: "1.46"
  003_change_env_variables_for_grafana_deployment: "1.51"



Comments:
// Migration: 1.45 (2023-05-28)
*/

func (s *Store) upMigrations(client k8s.Client) error {
	if !s.initialized {
		return errors.New("not initialized")
	}

	// TODO: think about migrations order

	for _, mig := range s.migrations {
		// TODO: check that migration was no applied in the CM
		// TODO: save `dirty` migration to the CM
		err := mig.Up(client)
		if err != nil {
			return err
		}

		// TODO: save `success` migration to the CM
	}

	// Check migrations, that are in the CM but was not touched with a loop - they are outdated(mark them for deletion)

	return nil
}

func (s *Store) downMigrations(desiredVersion string, client k8s.Client) error {
	if !s.initialized {
		return errors.New("not initialized")
	}
	// TODO: find migrations above the `desiredVersion` and Down() them

	//for _, mig := range s.migrations {
	//	mig.Down(client)
	//}

	// TODO: remove them from the CM

	return nil
}

func Initialize(client k8s.Client) {
	// TODO: load already applied migrations from the CM
	// TODO: load deckhouse release version: read from `deckhouse/version` for example
	//client.CoreV1().ConfigMaps(cmNamespace).Get(...)
}

func Register(migration migration) {
	store.addMigration(migration)
}

func UpMigrations(client k8s.Client) error {
	return store.upMigrations(client)
}

func DownMigrations(desiredVersion string, client k8s.Client) error {
	return store.downMigrations(desiredVersion, client)
}
