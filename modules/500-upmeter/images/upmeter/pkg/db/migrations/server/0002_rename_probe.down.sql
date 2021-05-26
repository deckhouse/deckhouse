BEGIN IMMEDIATE;

UPDATE downtime30s SET   probe_name="control-plane-manager"  WHERE probe_name="controller-manager";
UPDATE downtime5m  SET   probe_name="control-plane-manager"  WHERE probe_name="controller-manager";

COMMIT;
