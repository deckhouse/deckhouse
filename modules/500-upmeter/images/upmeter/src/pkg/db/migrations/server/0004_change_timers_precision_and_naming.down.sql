BEGIN IMMEDIATE;

-- Episodes 30s

UPDATE  episodes_30s
SET
    nano_up         = nano_up / 1e9,
    nano_down       = nano_down / 1e9,
    nano_unknown    = nano_unknown / 1e9,
    nano_unmeasured = nano_unmeasured / 1e9;

ALTER TABLE  episodes_30s  RENAME COLUMN  nano_up            TO  success_seconds;
ALTER TABLE  episodes_30s  RENAME COLUMN  nano_down          TO  fail_seconds;
ALTER TABLE  episodes_30s  RENAME COLUMN  nano_unknown       TO  unknown_seconds;
ALTER TABLE  episodes_30s  RENAME COLUMN  nano_unmeasured    TO  nodata_seconds;

ALTER TABLE  episodes_30s   RENAME TO downtime30s ;

-- Episodes 5m

UPDATE  episodes_5m
SET
    nano_up         = nano_up / 1e9,
    nano_down       = nano_down / 1e9,
    nano_unknown    = nano_unknown / 1e9,
    nano_unmeasured = nano_unmeasured / 1e9;

ALTER TABLE  episodes_5m  RENAME COLUMN  nano_up            TO  success_seconds;
ALTER TABLE  episodes_5m  RENAME COLUMN  nano_down          TO  fail_seconds;
ALTER TABLE  episodes_5m  RENAME COLUMN  nano_unknown       TO  unknown_seconds;
ALTER TABLE  episodes_5m  RENAME COLUMN  nano_unmeasured    TO  nodata_seconds;

ALTER TABLE   episodes_5m    RENAME TO  downtime5m;

-- Episodes to export

UPDATE  export_episodes
SET
    nano_up         = nano_up / 1e9,
    nano_down       = nano_down / 1e9,
    nano_unknown    = nano_unknown / 1e9,
    nano_unmeasured = nano_unmeasured / 1e9;

ALTER TABLE  export_episodes  RENAME COLUMN  nano_up            TO success  ;
ALTER TABLE  export_episodes  RENAME COLUMN  nano_down          TO fail  ;
ALTER TABLE  export_episodes  RENAME COLUMN  nano_unknown       TO unknown  ;
ALTER TABLE  export_episodes  RENAME COLUMN  nano_unmeasured    TO nodata  ;


COMMIT;
