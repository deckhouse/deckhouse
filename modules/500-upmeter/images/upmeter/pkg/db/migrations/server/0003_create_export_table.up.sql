/*

This migration creates the table for episodes to be exported as metrics via Prometheus remote_write protocol.
In addition to episode data, the table tracks the remote_write target by "sync_id" field, and also tracks the
fulfillment state of an episode in "origins" and "origins_count".

*/

BEGIN IMMEDIATE;

CREATE TABLE IF NOT EXISTS export_episodes
(
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


COMMIT;
