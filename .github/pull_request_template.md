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
type: fix | feature | chore
summary: <ONE LINE of what effectively changes for a user>
impact_level: low | high* <this is an impact for users, not deckhouse>
impact: <what to expect for users, possibly multi-line>, required if impact_level is high
```

<!---
Tip for the section field:

  - <kebab-case of a module>, e.g. "cloud-provider-aws", "node-manager"
  - "ci", has forced low impact
  - "docs", includes website changes, should have low impact
  - "candi"
  - "deckhouse-controller"
  - "dhctl"
  - "global-hooks"
  - "go_lib"
  - "helm_lib"
  - "jq_lib"
  - "shell_lib"
  - "testing", has forced low impact
  - "tools", has forced low impact

Find changed sections:

gh pr diff   $PULL_REQUEST_NUMBER   |
  egrep "([+]{3}|[-]{3}) [ab]/" |
  cut -d/ -f2- |
  sed 's#^ee/##' |
  sed 's#^fe/##' |
  sed 's#^modules/##' |
  sed 's#[0-9][0-9][0-9]-##' |
  cut -d/ -f1 |
  sort |
  uniq

Find all possible sections (excluding ci):

node -e 'console.log(require("./.github/scripts/js/changelog-find-sections.js")().join("\n"))'
-->
