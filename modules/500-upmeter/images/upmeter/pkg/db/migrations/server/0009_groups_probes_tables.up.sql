/*

 Normalize groups and probes

 */
BEGIN IMMEDIATE;

-- groups table
CREATE TABLE IF NOT EXISTS groups (
        id INTEGER NOT NULL PRIMARY KEY,
        name STRING NOT NULL UNIQUE
);

-- probes table
CREATE TABLE IF NOT EXISTS probes (
        id INTEGER NOT NULL PRIMARY KEY,
        name STRING NOT NULL,
        group_id INTEGER NOT NULL REFERENCES groups (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_probe_per_group ON probes (group_id, name);

-- fill groups
INSERT
        OR IGNORE INTO groups (name)
SELECT
        DISTINCT group_name
FROM
        episodes_5m;

-- fill probes
INSERT
        OR IGNORE INTO probes (name, group_id)
SELECT
        DISTINCT episodes_5m.probe_name,
        groups.id
FROM
        episodes_5m
        INNER JOIN groups ON groups.name = episodes_5m.group_name;

-- add probe reference to 30s episodes
-- ALTER TABLE
--         users
-- ADD
--         COLUMN dayChoice_id INTEGER NOT NULL REFERENCES dayChoice(dayChoice_id) DEFAULT 0;
PRAGMA foreign_keys = 0;

ALTER TABLE
        episodes_30s
ADD
        -- COLUMN probe_id INTEGER NOT NULL REFERENCES probes (id)    DEFAULT 0    ON DELETE CASCADE ON UPDATE CASCADE;
        COLUMN probe_id INTEGER NOT NULL REFERENCES probes (id) ON DELETE CASCADE ON UPDATE CASCADE;

UPDATE
        episodes_30s
SET
        probe_id = (
                SELECT
                        id
                FROM
                        probes
                        INNER JOIN groups ON groups.id = probes.group_id
                WHERE
                        probes.name = episodes_30s.probe_name
        );

PRAGMA foreign_keys = 1;

-- add probe reference to 5m episodes
-- add probe reference to exported episodes
-- remvoe redundant columns from 30s episodes
-- remvoe redundant columns from 5m episodes
-- remvoe redundant columns from exported episodes
COMMIT;
