package migrations

var v0000_actions = []map[string]string{
	{
		"desc": "ensure table downtime30s",
		"sql": `
CREATE TABLE IF NOT EXISTS downtime30s (
	timeslot INTEGER NOT NULL,
    success_seconds INTEGER NOT NULL,
    fail_seconds INTEGER NOT NULL,
    group_name TEXT NOT NULL,
    probe_name TEXT NOT NULL
)
`,
	},
	{
		"desc": "ensure table downtime5m",
		"sql": `
CREATE TABLE IF NOT EXISTS downtime5m (
	timeslot INTEGER NOT NULL,
    success_seconds INTEGER NOT NULL,
    fail_seconds INTEGER NOT NULL,
    group_name TEXT NOT NULL,
    probe_name TEXT NOT NULL
)
`,
	},
}

func V0000_Up(m *MigratorService) {
	m.applyActions("V0000", v0000_actions)
}
