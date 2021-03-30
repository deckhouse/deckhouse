package migrations

import "fmt"

func V0003_Up(m *MigratorService) {
	forTable := func(table string) string {
		return fmt.Sprintf(`
			UPDATE %s 
			SET   probe_name="controller-manager"
			WHERE probe_name="control-plane-manager";
		`,
			table)
	}

	m.applyActions("V0003",
		[]map[string]string{
			{
				"desc": "rename in episodes 30s",
				"sql":  forTable("downtime30s"),
			}, {
				"desc": "rename in episodes 5m",
				"sql":  forTable("downtime5m"),
			},
		},
	)
}
