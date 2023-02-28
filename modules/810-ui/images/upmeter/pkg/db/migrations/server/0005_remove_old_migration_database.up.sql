/*

This migration cleans the old migration table since it is obsolete.

*/

BEGIN IMMEDIATE;

DROP TABLE IF EXISTS _schema_version;


COMMIT;
