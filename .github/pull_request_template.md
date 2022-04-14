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
section: <kebab-case of a module name> | <1st level dir in the repo>
type: fix | feature | chore
summary: <ONE-LINE of what effectively changes for a user>
impact: <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
impact_level: default | high | low
```

<!---
`impact_level: default` adds to changelog as usual, this is the default that can be omitted
`impact_level: high`    something important for users, the impact will be copied to "Know Before Update" section
`impact_level: low`     omitted in changelog YAML; note there is `type:chore` for chores

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
  egrep "^([+]{3} b|[-]{3} a)/" |
  cut -d/ -f2- |
  sed 's#^ee/##' |
  sed 's#^fe/##' |
  sed 's#^modules/##' |
  sed 's#[0-9][0-9][0-9]-##' |
  egrep -v 'Makefile' |       # add file exclusion here
  cut -d/ -f1 |
  sort |
  uniq

Find all possible sections (excluding ci):

node -e 'console.log(require("./.github/scripts/js/changelog-find-sections.js")().join("\n"))'
-->
