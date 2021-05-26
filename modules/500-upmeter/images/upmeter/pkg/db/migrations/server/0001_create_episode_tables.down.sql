BEGIN IMMEDIATE;

DROP INDEX downtime5m_time_group_probe;
DROP INDEX downtime30s_time_group_probe;

DROP TABLE IF EXISTS downtime30s;
DROP TABLE IF EXISTS downtime5m;

COMMIT;
