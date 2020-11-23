package migrations

var v0001_actions = []map[string]string{
	{
		"desc": "downtime30s add unknown_seconds",
		"sql":  `alter table downtime30s add column unknown_seconds integer not null default 0`,
	},
	{
		"desc": "downtime30s add nodata_seconds",
		"sql":  `alter table downtime30s add column nodata_seconds integer not null default 0`,
	},
	{
		"desc": "downtime5m add unknown_seconds",
		"sql":  `alter table downtime5m add column unknown_seconds integer not null default 0`,
	},
	{
		"desc": "downtime5m add nodata_seconds",
		"sql":  `alter table downtime5m add column nodata_seconds integer not null default 0`,
	},
	{
		"desc": "downtime30s update unknown_seconds",
		"sql":  `update downtime30s set unknown_seconds=(30-(success_seconds+fail_seconds))`,
	},
	{
		"desc": "downtime5m update unknown_seconds",
		"sql":  `update downtime5m set unknown_seconds=(300-(success_seconds+fail_seconds))`,
	},
}

func V0001_Up(m *MigratorService) {
	m.applyActions("V0001", v0001_actions)
}
