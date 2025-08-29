# Changelog v1.73

## Features


 - **[deckhouse-controller]** add new objects to debug archive [#15047](https://github.com/deckhouse/deckhouse/pull/15047)
 - **[deckhouse-controller]** task queue performance improvements with linked list implementation [#14962](https://github.com/deckhouse/deckhouse/pull/14962)
 - **[deckhouse-controller]** handle errors while processing source modules [#14953](https://github.com/deckhouse/deckhouse/pull/14953)

## Fixes


 - **[cloud-provider-vcd]** Fix fetching VM templates from organization catalogs without direct access to organizastion [#14980](https://github.com/deckhouse/deckhouse/pull/14980)
 - **[deckhouse-controller]** fixed bug with re-enabled module using old values [#15045](https://github.com/deckhouse/deckhouse/pull/15045)
 - **[deckhouse-controller]** Implement structured releaseQueueDepth calculation with hierarchical version delta tracking [#15031](https://github.com/deckhouse/deckhouse/pull/15031)
    The releaseQueueDepth metric now accurately reflects actionable release gaps with patch version normalization; major version tracking added for future alerting
 - **[user-authn]** User now can't create groups with  recursive loops in nested group's hierarchy [#15139](https://github.com/deckhouse/deckhouse/pull/15139)

## Chore


 - **[admission-policy-engine]** Fix CVE's [#15237](https://github.com/deckhouse/deckhouse/pull/15237)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.16.0. [#15111](https://github.com/deckhouse/deckhouse/pull/15111)
 - **[registry]** Fixed CVE's: CVE-2020-26160, CVE-2020-8911, CVE-2020-8912, CVE-2022-21698, CVE-2022-2582, CVE-2025-22868, CVE-2025-22869, CVE-2025-22870, CVE-2025-22872, CVE-2025-27144 [#15235](https://github.com/deckhouse/deckhouse/pull/15235)

