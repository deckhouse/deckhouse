/*

 Clean group stats.

 */
BEGIN IMMEDIATE;

DELETE FROM
        episodes_5m
WHERE
        probe_name == "__total__";

COMMIT;
