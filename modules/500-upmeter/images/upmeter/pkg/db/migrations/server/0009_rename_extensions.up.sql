/*

- Rename "scaling" group to "extensions"
- Rename control-plane/access probe to control-plane/apiserver

https://github.com/deckhouse/deckhouse/issues/1532

*/

BEGIN IMMEDIATE;

UPDATE downtime30s     SET   group_name="extensions"  WHERE group_name="scaling";
UPDATE downtime5m      SET   group_name="extensions"  WHERE group_name="scaling";
UPDATE export_episodes SET   group_name="extensions"  WHERE group_name="scaling";

UPDATE downtime30s     SET   probe_name="apiserver"  WHERE group_name="control-plane" AND probe_name="access";
UPDATE downtime5m      SET   probe_name="apiserver"  WHERE group_name="control-plane" AND probe_name="access";
UPDATE export_episodes SET   probe_name="apiserver"  WHERE group_name="control-plane" AND probe_name="access";

COMMIT;
