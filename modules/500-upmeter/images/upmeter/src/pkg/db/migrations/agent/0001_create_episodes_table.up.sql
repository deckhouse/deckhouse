/*

This migration adds the table for episodes to store before sending to the server.

Current DAO implementation for 30s episodes uses the same schema and table name, so we reproduce the schema
from the server.

*/


BEGIN IMMEDIATE;

CREATE TABLE IF NOT EXISTS "episodes_30s"
(
    timeslot        INTEGER NOT NULL,

    group_name      TEXT    NOT NULL,
    probe_name      TEXT    NOT NULL,

    nano_up         INTEGER NOT NULL,
    nano_down       INTEGER NOT NULL,
    nano_unknown    INTEGER NOT NULL,
    nano_unmeasured INTEGER NOT NULL
);

CREATE UNIQUE INDEX episodes30s_time_group_probe on "episodes_30s" (timeslot, group_name, probe_name);

COMMIT;
