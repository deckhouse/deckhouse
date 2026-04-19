-- This table is useless in the migration rollback, but at least we restore the schema

BEGIN IMMEDIATE;

CREATE TABLE IF NOT EXISTS _schema_version
(
    timestamp INTEGER NOT NULL,
    version   TEXT    NOT NULL
);


COMMIT;
