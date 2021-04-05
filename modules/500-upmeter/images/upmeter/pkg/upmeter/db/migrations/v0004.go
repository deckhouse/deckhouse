package migrations

func V0004_Up(m *MigratorService) {
	m.applyActions("V0004",
		[]map[string]string{{
			"desc": "create table for export episodes",
			"sql": `
CREATE TABLE IF NOT EXISTS export_episodes (
        sync_id       TEXT    NOT NULL,
	timeslot      INTEGER NOT NULL,
	group_name    TEXT    NOT NULL,
	probe_name    TEXT    NOT NULL,
	success       INTEGER NOT NULL,
	fail          INTEGER NOT NULL,
	unknown       INTEGER NOT NULL,
	nodata        INTEGER NOT NULL,
	origins       TEXT    NOT NULL,
	origins_count INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS sync_id_sorted ON export_episodes (sync_id, timeslot, group_name, probe_name);
`,
		}},
	)
}
