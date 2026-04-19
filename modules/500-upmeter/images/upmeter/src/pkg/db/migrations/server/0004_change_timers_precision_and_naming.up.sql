/*

This migration converts episode timers to high precision — from seconds to nanoseconds, and renames the columns
accordingly. Also, tables are renamed to emphasis the entity of an episode, not just "downtime".

The precision of nanoseconds is chosen to conveniently convert the data to time.Duration.

*/


BEGIN IMMEDIATE;

-- Cleanup

DELETE FROM
    downtime30s
WHERE
        timeslot < (
        SELECT
            min(timeslot)-300 -- offset by 300s guarantees to take only for fulfilled 5m episodes
        FROM
            (
                SELECT
                    max(timeslot) as timeslot
                FROM
                    downtime5m
                GROUP BY
                    group_name,
                    probe_name
            )
    );


COMMIT;


-- Vacuum is not allowed within a transaction
VACUUM;




BEGIN IMMEDIATE ;

-- Episodes 30s

ALTER TABLE  downtime30s  RENAME TO  episodes_30s;

ALTER TABLE  episodes_30s  RENAME COLUMN  success_seconds  TO  nano_up;
ALTER TABLE  episodes_30s  RENAME COLUMN  fail_seconds     TO  nano_down;
ALTER TABLE  episodes_30s  RENAME COLUMN  unknown_seconds  TO  nano_unknown;
ALTER TABLE  episodes_30s  RENAME COLUMN  nodata_seconds   TO  nano_unmeasured;

UPDATE  episodes_30s
SET
    nano_up         = 1e9 * nano_up,
    nano_down       = 1e9 * nano_down,
    nano_unknown    = 1e9 * nano_unknown,
    nano_unmeasured = 1e9 * nano_unmeasured;


-- Episodes 5m

ALTER TABLE  downtime5m  RENAME TO  episodes_5m;

ALTER TABLE  episodes_5m  RENAME COLUMN  success_seconds  TO  nano_up;
ALTER TABLE  episodes_5m  RENAME COLUMN  fail_seconds     TO  nano_down;
ALTER TABLE  episodes_5m  RENAME COLUMN  unknown_seconds  TO  nano_unknown;
ALTER TABLE  episodes_5m  RENAME COLUMN  nodata_seconds   TO  nano_unmeasured;

UPDATE  episodes_5m
SET
    nano_up         = 1e9 * nano_up,
    nano_down       = 1e9 * nano_down,
    nano_unknown    = 1e9 * nano_unknown,
    nano_unmeasured = 1e9 * nano_unmeasured;


-- Episodes to export

ALTER TABLE  export_episodes  RENAME COLUMN  success  TO  nano_up;
ALTER TABLE  export_episodes  RENAME COLUMN  fail     TO  nano_down;
ALTER TABLE  export_episodes  RENAME COLUMN  unknown  TO  nano_unknown;
ALTER TABLE  export_episodes  RENAME COLUMN  nodata   TO  nano_unmeasured;

UPDATE  export_episodes
SET
    nano_up         = 1e9 * nano_up,
    nano_down       = 1e9 * nano_down,
    nano_unknown    = 1e9 * nano_unknown,
    nano_unmeasured = 1e9 * nano_unmeasured;


COMMIT;
