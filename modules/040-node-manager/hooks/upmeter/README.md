# upmeter

Bridge to the upmeter module, owned by the observability team.

- `upmeter_discovery` selects the CloudEphemeral NodeGroups that must always keep at least one ready node: min per zone is at least 1, and the allowed number of unavailable nodes leaves at least one. It publishes their names in a ConfigMap, and upmeter runs availability probes for those groups.
