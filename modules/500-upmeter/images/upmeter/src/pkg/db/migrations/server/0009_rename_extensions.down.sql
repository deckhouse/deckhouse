/*

- Revert renaming "scaling" group to "extensions" for current two probes
- Revert rename of control-plane/access probe to control-plane/apiserver

https://github.com/deckhouse/deckhouse/issues/1532

*/

BEGIN IMMEDIATE;

UPDATE episodes_30s    SET   group_name="scaling"  WHERE group_name="extensions" AND (probe_name="cluster-scaling" OR probe_name="cluster-autoscaler");
UPDATE episodes_5m     SET   group_name="scaling"  WHERE group_name="extensions" AND (probe_name="cluster-scaling" OR probe_name="cluster-autoscaler");
UPDATE export_episodes SET   group_name="scaling"  WHERE group_name="extensions" AND (probe_name="cluster-scaling" OR probe_name="cluster-autoscaler");

UPDATE episodes_30s    SET   probe_name="access"  WHERE group_name="control-plane" AND probe_name="apiserver";
UPDATE episodes_5m     SET   probe_name="access"  WHERE group_name="control-plane" AND probe_name="apiserver";
UPDATE export_episodes SET   probe_name="access"  WHERE group_name="control-plane" AND probe_name="apiserver";

COMMIT;
