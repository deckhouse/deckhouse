/*

By mistake we had probe name "control-plane-manager", we fix it by renaming the presence in all records to
"controller-manager".

*/

BEGIN IMMEDIATE;

UPDATE downtime30s SET   probe_name="controller-manager"  WHERE probe_name="control-plane-manager";
UPDATE downtime5m  SET   probe_name="controller-manager"  WHERE probe_name="control-plane-manager";

COMMIT;
