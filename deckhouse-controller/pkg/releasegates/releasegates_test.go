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

package releasegates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/d8sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMigration(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		ok      bool
		version int
		dir     Direction
	}{
		{name: "up", file: "001_add_index.up.sql", ok: true, version: 1, dir: DirectionUp},
		{name: "down", file: "001_add_index.down.sql", ok: true, version: 1, dir: DirectionDown},
		{name: "multi digit", file: "0042_drop_legacy_secret.up.sql", ok: true, version: 42, dir: DirectionUp},
		{name: "underscores in name", file: "7_a_b_c.down.sql", ok: true, version: 7, dir: DirectionDown},
		{name: "no direction", file: "001_add_index.sql", ok: false},
		{name: "no version", file: "add_index.up.sql", ok: false},
		{name: "no separator", file: "001.up.sql", ok: false},
		{name: "not a migration", file: "README.md", ok: false},
		{name: "negative version", file: "-1_x.up.sql", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration, ok := parseMigration(tt.file)
			require.Equal(t, tt.ok, ok)
			if !tt.ok {
				return
			}

			assert.Equal(t, tt.version, migration.Version)
			assert.Equal(t, tt.dir, migration.Direction)
			assert.Equal(t, tt.file, migration.Name)
		})
	}
}

func TestMigrationsOrder(t *testing.T) {
	modulePath := t.TempDir()
	writeFiles(t, MigrationsDir(modulePath), map[string]string{
		"010_ten.up.sql":    "",
		"010_ten.down.sql":  "",
		"2_two.up.sql":      "",
		"1_one.up.sql":      "",
		"1_one.down.sql":    "",
		"notes.txt":         "",
		"broken.up.sql":     "",
		"3_three.up.sql.gz": "",
	})

	migrations, err := Migrations(modulePath)
	require.NoError(t, err)

	got := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		got = append(got, migration.Name)
	}

	// numeric order, not lexical: 2 before 10
	assert.Equal(t, []string{
		"1_one.down.sql", "1_one.up.sql",
		"2_two.up.sql",
		"010_ten.down.sql", "010_ten.up.sql",
	}, got)
}

func TestMigrationsMissingDir(t *testing.T) {
	migrations, err := Migrations(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, migrations)

	validations, err := Validations(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, validations)
}

func TestValidationsLexicalOrder(t *testing.T) {
	modulePath := t.TempDir()
	writeFiles(t, ValidationsDir(modulePath), map[string]string{
		"20_edition.sql": "",
		"10_nodes.sql":   "",
		"readme.md":      "",
	})

	files, err := Validations(modulePath)
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "10_nodes.sql", filepath.Base(files[0]))
	assert.Equal(t, "20_edition.sql", filepath.Base(files[1]))
}

func TestPending(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Name: "1.up.sql", Direction: DirectionUp},
		{Version: 1, Name: "1.down.sql", Direction: DirectionDown},
		{Version: 2, Name: "2.up.sql", Direction: DirectionUp},
		{Version: 3, Name: "3.up.sql", Direction: DirectionUp},
		{Version: 3, Name: "3.down.sql", Direction: DirectionDown},
	}

	tests := []struct {
		name    string
		applied int
		want    []string
	}{
		{name: "first install", applied: 0, want: []string{"1.up.sql", "2.up.sql", "3.up.sql"}},
		{name: "partially applied", applied: 1, want: []string{"2.up.sql", "3.up.sql"}},
		{name: "up to date", applied: 3, want: nil},
		{name: "applied ahead", applied: 9, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, names(PendingUp(migrations, tt.applied)))
		})
	}

	// downgrade: everything above the target version, newest first
	assert.Equal(t, []string{"3.down.sql", "1.down.sql"}, names(PendingDown(migrations, 0)))
	assert.Equal(t, []string{"3.down.sql"}, names(PendingDown(migrations, 1)))
	assert.Nil(t, names(PendingDown(migrations, 3)))

	assert.Equal(t, 3, MaxVersion(migrations))
	assert.Equal(t, 0, MaxVersion(nil))
}

func TestRunAgainstPlatformTable(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"edition.sql": "ASSERT NOT EMPTY (SELECT deckhouseEdition FROM " + PlatformTable +
			" WHERE deckhouseEdition IN ('ee','fe')) FAIL 'EDITION' 'the module requires EE';",
		"empty.sql": "\n\n",
	})

	// no cluster is touched: the gate only reads the virtual table
	engine := d8sql.New(nil, nil, Platform{DeckhouseEdition: "ce", KubernetesVersion: "1.30.2"}.Option())

	err := Run(t.Context(), engine, filepath.Join(dir, "edition.sql"))
	require.Error(t, err)

	var validationErr *d8sql.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "EDITION", validationErr.Code)
	assert.Contains(t, err.Error(), "edition.sql")

	require.NoError(t, Run(t.Context(), engine, filepath.Join(dir, "empty.sql")))

	engine = d8sql.New(nil, nil, Platform{DeckhouseEdition: "ee"}.Option())
	require.NoError(t, Run(t.Context(), engine, filepath.Join(dir, "edition.sql")))
}

func names(migrations []Migration) []string {
	if len(migrations) == 0 {
		return nil
	}

	got := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		got = append(got, migration.Name)
	}

	return got
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
}
