/*

This migration create initial tables to store temporary episodes (30s) and long-term stored episodes (5m).
Indexes are intended to keep episodes unique by time slot and probe.

NOTE: This migration stores the state of the database schema in 2023.01 release since we reimplemented migrations
with golang-migrations

*/

BEGIN IMMEDIATE;

CREATE TABLE IF NOT EXISTS downtime30s
(
    timeslot        INTEGER NOT NULL,
    success_seconds INTEGER NOT NULL,
    fail_seconds    INTEGER NOT NULL,
    group_name      TEXT    NOT NULL,
    probe_name      TEXT    NOT NULL,
    unknown_seconds INTEGER NOT NULL DEFAULT 0,
    nodata_seconds  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS downtime5m
(
    timeslot        INTEGER NOT NULL,
    success_seconds INTEGER NOT NULL,
    fail_seconds    INTEGER NOT NULL,
    group_name      TEXT    NOT NULL,
    probe_name      TEXT    NOT NULL,
    unknown_seconds INTEGER NOT NULL DEFAULT 0,
    nodata_seconds  INTEGER NOT NULL DEFAULT 0
);


CREATE UNIQUE INDEX IF NOT EXISTS downtime30s_time_group_probe ON downtime30s (timeslot, group_name, probe_name);
CREATE UNIQUE INDEX IF NOT EXISTS downtime5m_time_group_probe  ON downtime5m  (timeslot, group_name, probe_name);

COMMIT ;
