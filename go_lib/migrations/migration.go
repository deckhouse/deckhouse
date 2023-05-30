package migrations

import "github.com/deckhouse/deckhouse/go_lib/dependency/k8s"

type DeckhouseMigration struct {
	name string
	up   MigrationFunc
	down MigrationFunc
}

type MigrationFunc func(client k8s.Client) error

func NewDeckhouseMigration(name string, up MigrationFunc, down MigrationFunc) DeckhouseMigration {
	return DeckhouseMigration{
		name: name,
		up:   up,
		down: down,
	}
}

func (dm DeckhouseMigration) Name() string {
	return dm.name
}

func (dm DeckhouseMigration) Up(client k8s.Client) error {
	return dm.up(client)
}

func (dm DeckhouseMigration) Down(client k8s.Client) error {
	return dm.down(client)
}
