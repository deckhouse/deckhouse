/*

This attempts to save disk space by reducing the database file size

*/

BEGIN IMMEDIATE;

DELETE FROM
    episodes_30s
WHERE
        timeslot < (
        SELECT
            min(timeslot)-300 -- offset by 300s guarantees to take only for fulfilled 5m episodes
        FROM
            (
                SELECT
                    max(timeslot) as timeslot
                FROM
                    episodes_5m
                GROUP BY
                    group_name,
                    probe_name
            )
    );


COMMIT;
