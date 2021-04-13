UPDATE downtime30s SET   probe_name="controller-manager"  WHERE probe_name="control-plane-manager";
UPDATE downtime5m  SET   probe_name="controller-manager"  WHERE probe_name="control-plane-manager";
