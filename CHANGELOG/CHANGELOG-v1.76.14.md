# Changelog v1.76.14

## Know before update


 - During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.

## Fixes


 - **[service-with-healthchecks]** Fixed a load balancer losing all its endpoints for good after a target pod was replaced. [#22980](https://github.com/deckhouse/deckhouse/pull/22980)
    The `service-with-healthchecks` controller and agent are restarted.
    A `ServiceWithHealthchecks` that was left in `NotAllEndpointsAreReady` with no EndpointSlice
    recovers automatically once the updated agent starts, no manual action is needed.
 - **[user-authn]** Fixed DexAuthenticator pod creation under ResourceQuota by setting init container CPU/memory limits to the sum of main container limits. [#22998](https://github.com/deckhouse/deckhouse/pull/22998)
 - **[user-authz]** Grant custom ClusterRoles (annotated with `user-authz.deckhouse.io/access-level`) through one aggregated ClusterRole per access level, so the number of bindings per ClusterAuthorizationRule/AuthorizationRule no longer depends on the number of such roles. [#22968](https://github.com/deckhouse/deckhouse/pull/22968)
    During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.
 - **[user-authz]** Group the module alerts under a group named apart from the alerts themselves, so the incident shelf accepts them. [#22999](https://github.com/deckhouse/deckhouse/pull/22999)

## Chore


 - **[deckhouse-controller]** bump nelm v1.30.3 [#23001](https://github.com/deckhouse/deckhouse/pull/23001)
 - **[prometheus]** Pin goyacc build dependency to a Go 1.25-compatible version to fix the image build. [#22983](https://github.com/deckhouse/deckhouse/pull/22983)
    prometheus
