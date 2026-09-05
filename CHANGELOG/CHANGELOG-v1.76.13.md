# Changelog v1.76.13

## Fixes


 - **[deckhouse-controller]** Honor a channel-level release suspend for clusters that reach the suspended version through a step-by-step update. [#22746](https://github.com/deckhouse/deckhouse/pull/22746)
    A Deckhouse release suspended on its release channel is no longer applied by clusters that are behind and reach it through a step-by-step update. The suspend flag lives only in the release-channel image; previously it was dropped when the target release was built from its per-version image, so lagging clusters updated to a suspended release anyway.
 - **[node-manager]** Fix NodeCapacity calculation. [#22677](https://github.com/deckhouse/deckhouse/pull/22677)
