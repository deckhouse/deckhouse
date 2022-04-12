/*

 Fulfill group stats (probe = "__total__") that were previously calculated on the fly.

 */
BEGIN IMMEDIATE;

-- Dummy calculation of a group uptime. It is pessimistic from the point of single probe episode,
-- but at the same time it is optimistic from the group perspective, since it implies all dowtime
-- probe fully overlaps in time.
INSERT
        OR IGNORE INTO episodes_5m (
                timeslot,
                nano_up,
                nano_down,
                nano_unknown,
                nano_unmeasured,
                group_name,
                probe_name
        )
SELECT
        timeslot,
        (
                -- This expression cannot evalueate to negative value. Maximum possible 'down' can
                -- be 3e11 iff minimum values of 'unknown' and 'unmeasured' are zeroes.
                300000000000 - MAX(nano_down) - MIN(nano_unknown) - MIN(nano_unmeasured)
        ) as nano_up,
        MAX(nano_down) as nano_down,
        MIN(nano_unknown) as nano_unknown,
        MIN(nano_unmeasured) as nano_unmeasured,
        group_name,
        "__total__"
FROM
        episodes_5m
GROUP BY
        timeslot,
        group_name;

COMMIT;
