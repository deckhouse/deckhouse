## Description
<!---
  Describe your changes in detail.

  Please let users know if your feature influences critical cluster components
  (restarts of ingress-controllers, control-plane, Prometheus, etc).
-->

## Why do we need it, and what problem does it solve?
<!---
  This is the most important paragraph.
  You must describe the main goal of your feature.

  If it fixes an issue, place a link to the issue here.

  If it fixes an obvious bug, please tell users about the impact and effect of the problem.
-->

## Changelog entries
<!---
  Describe the changes so they will be included in a release changelog.

  Find examples and documentation below, or visit the instruction page on the repo wiki
  https://github.com/deckhouse/deckhouse/wiki/How-to-add-to-changelog
-->

```changes
section: <kebab-case of a modules/*> | <1st level dir in the repo>
type: fix | feature
summary: <what effectively changes in a single line>
impact_level: low | high*
impact: <what to expect, possibly multi-line>, required if impact_level is high
```

<!---
Tip for the section field:

  - <kebab-case of a modules/*>, like "cloud-provider-aws", "node-manager"
  - "dhctl"
  - "candi"
  - "deckhouse-controller"
  - *_lib
  - "docs", includes website changes, should always have low impact
  - "tests", should always have low impact
  - "tools", should always have low impact
  - "ci", should always have low impact
  - "global" affects all possible modules at once, discouraged if only a few of modules affected, it is better to have multiple exact changes

-->
