// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package releasegates discovers and runs the SQL gates shipped with a module
// release: validations executed before the module is installed and migrations
// applied on upgrade (or rolled back on downgrade).
//
// The module layout is:
//
//	<module>/release/validations/*.sql              executed in lexical order
//	<module>/release/migrations/NNN_<name>.up.sql   golang-migrate style
//	<module>/release/migrations/NNN_<name>.down.sql
package releasegates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/d8sql"
	"github.com/deckhouse/d8sql/sql"
)

const (
	// PlatformTable is the virtual table exposing the platform facts to gates.
	PlatformTable = "v_d8_platform"

	validationsSubdir = "release/validations"
	migrationsSubdir  = "release/migrations"

	upSuffix   = ".up.sql"
	downSuffix = ".down.sql"
)

// Platform holds the facts exposed to gates through the PlatformTable virtual
// table, so a gate can branch on the cluster it is about to be applied to.
type Platform struct {
	DeckhouseVersion  string
	DeckhouseEdition  string
	DeckhouseBundle   string
	KubernetesVersion string
}

// NormalizeVersion turns a kubernetes GitVersion (v1.30.2+d8) into a bare
// semver (1.30.2) so gates can compare it as a plain string.
func NormalizeVersion(gitVersion string) string {
	parsed, err := semver.NewVersion(gitVersion)
	if err != nil {
		return strings.TrimPrefix(gitVersion, "v")
	}

	return parsed.String()
}

// Option returns the engine option registering the platform virtual table.
func (p Platform) Option() d8sql.Option {
	return d8sql.WithVirtualTable(PlatformTable, []map[string]any{{
		"deckhouseVersion":  p.DeckhouseVersion,
		"deckhouseEdition":  p.DeckhouseEdition,
		"deckhouseBundle":   p.DeckhouseBundle,
		"kubernetesVersion": p.KubernetesVersion,
	}})
}

// Direction of a migration.
type Direction string

const (
	DirectionUp   Direction = "Up"
	DirectionDown Direction = "Down"
)

// Migration is one parsed migration file.
type Migration struct {
	// Version is the numeric NNN prefix of the file name.
	Version int
	// Name is the file base name, e.g. "001_add_index.up.sql".
	Name string
	// Path is the absolute path of the file.
	Path string
	// Direction is derived from the .up.sql/.down.sql suffix.
	Direction Direction
}

// ValidationsDir returns the validations directory of an unpacked module.
func ValidationsDir(modulePath string) string {
	return filepath.Join(modulePath, validationsSubdir)
}

// MigrationsDir returns the migrations directory of an unpacked module.
func MigrationsDir(modulePath string) string {
	return filepath.Join(modulePath, migrationsSubdir)
}

// Validations lists the validation files of a module in lexical order. A
// missing directory is not an error: modules without gates return no files.
func Validations(modulePath string) ([]string, error) {
	entries, err := os.ReadDir(ValidationsDir(modulePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read validations dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		files = append(files, filepath.Join(ValidationsDir(modulePath), entry.Name()))
	}
	sort.Strings(files)

	return files, nil
}

// Migrations lists the migrations of a module ordered by version ascending
// (up before down within the same version). A missing directory is not an
// error. Files not matching NNN_<name>.(up|down).sql are ignored, up and down
// counterparts do not have to come in pairs.
func Migrations(modulePath string) ([]Migration, error) {
	dir := MigrationsDir(modulePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		migration, ok := parseMigration(entry.Name())
		if !ok {
			continue
		}

		migration.Path = filepath.Join(dir, entry.Name())
		migrations = append(migrations, migration)
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].Version != migrations[j].Version {
			return migrations[i].Version < migrations[j].Version
		}

		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

func parseMigration(name string) (Migration, bool) {
	direction := DirectionUp
	switch {
	case strings.HasSuffix(name, upSuffix):
	case strings.HasSuffix(name, downSuffix):
		direction = DirectionDown
	default:
		return Migration{}, false
	}

	prefix, _, found := strings.Cut(name, "_")
	if !found || prefix == "" {
		return Migration{}, false
	}

	version, err := strconv.Atoi(prefix)
	if err != nil || version < 0 {
		return Migration{}, false
	}

	return Migration{Version: version, Name: name, Direction: direction}, true
}

// PendingUp returns the up migrations to apply on top of the applied version,
// ascending. maxApplied is the highest already applied version, 0 when nothing
// was applied yet (migration versions therefore start at 1).
func PendingUp(migrations []Migration, maxApplied int) []Migration {
	pending := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if migration.Direction == DirectionUp && migration.Version > maxApplied {
			pending = append(pending, migration)
		}
	}

	return pending
}

// PendingDown returns the down migrations to roll back to targetVersion,
// descending: everything applied above the version the module is downgraded to.
func PendingDown(migrations []Migration, targetVersion int) []Migration {
	pending := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if migration.Direction == DirectionDown && migration.Version > targetVersion {
			pending = append(pending, migration)
		}
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].Version > pending[j].Version })

	return pending
}

// MaxVersion returns the highest migration version, 0 when there is none.
func MaxVersion(migrations []Migration) int {
	maxVersion := 0
	for _, migration := range migrations {
		if migration.Version > maxVersion {
			maxVersion = migration.Version
		}
	}

	return maxVersion
}

// Run executes every statement of a single SQL file and reports how many objects
// it changed. The whole file is prepared before anything runs, so a syntax error
// never leaves a half-applied batch behind.
//
// The count matters for migrations: a statement that matches nothing is a
// successful no-op, so without it a migration that quietly did nothing looks
// exactly like one that worked.
func Run(ctx context.Context, engine *d8sql.Engine, path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %q: %w", filepath.Base(path), err)
	}

	if strings.TrimSpace(string(content)) == "" {
		return 0, nil
	}

	results, err := engine.Execute(ctx, string(content))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	return affected(results), nil
}

// affected sums the objects changed by a batch, descending into the statements
// an IF branch executed. Only mutating statements count: ASSERT reports the
// number of objects it matched in the same field, and those were read, not
// changed.
func affected(results []d8sql.Result) int {
	total := 0
	for _, result := range results {
		switch result.Kind {
		case sql.StmtUpdate, sql.StmtDelete:
			total += result.Affected
		}

		total += affected(result.Nested)
	}

	return total
}
