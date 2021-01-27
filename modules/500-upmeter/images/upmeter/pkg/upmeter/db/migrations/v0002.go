package migrations

var v0002_actions = []map[string]string{
	{
		"desc": "index for downtime5m",
		"sql":  `create index downtime5m_time_group_probe on downtime5m(timeslot, group_name, probe_name)`,
	},
	{
		"desc": "index for downtime30s",
		"sql":  `create index downtime30s_time_group_probe on downtime30s(timeslot, group_name, probe_name)`,
	},
}

func V0002_Up(m *MigratorService) {
	m.applyActions("V0002", v0002_actions)
}
